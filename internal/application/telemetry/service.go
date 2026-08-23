package telemetryapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	FindCrane(context.Context, string) (site.TowerCrane, error)
	FindSensor(context.Context, string) (sensor.Sensor, error)
	FindCalibration(context.Context, string, int64) (sensor.Calibration, error)
	LastSequence(context.Context, string) (int64, error)
	EventExists(context.Context, string, string) (bool, error)
	StoreReading(context.Context, telemetry.NormalizedReading) error
	StoreQuarantined(context.Context, telemetry.RawReading, string) error
	RecentReadings(context.Context, string, time.Time) ([]telemetry.NormalizedReading, error)
}

type UnitConverter interface {
	Convert(kind sensor.Kind, value int64, from, to string) (int64, error)
}

type Service struct {
	repository Repository
	converter  UnitConverter
	clock      clock.Clock
	maxFuture  time.Duration
	outOfOrder int64
	staleAfter time.Duration
}

func New(repository Repository, converter UnitConverter, c clock.Clock) *Service {
	return &Service{repository: repository, converter: converter, clock: c, maxFuture: 5 * time.Second, outOfOrder: 3, staleAfter: 10 * time.Second}
}

func (s *Service) Ingest(ctx context.Context, raw telemetry.RawReading) (telemetry.NormalizedReading, error) {
	now := s.clock.Now()
	raw.ReceivedAt = now
	if err := raw.Validate(now, s.maxFuture); err != nil {
		_ = s.repository.StoreQuarantined(ctx, raw, err.Error())
		return telemetry.NormalizedReading{}, err
	}
	crane, err := s.repository.FindCrane(ctx, raw.CraneID)
	if err != nil {
		return telemetry.NormalizedReading{}, err
	}
	device, err := s.repository.FindSensor(ctx, raw.SensorID)
	if err != nil || device.CraneID != raw.CraneID || device.Kind != raw.Kind {
		_ = s.repository.StoreQuarantined(ctx, raw, "sensor binding mismatch")
		return telemetry.NormalizedReading{}, errors.New("sensor binding mismatch")
	}
	exists, err := s.repository.EventExists(ctx, raw.CraneID, raw.EventID)
	if err != nil {
		return telemetry.NormalizedReading{}, err
	}
	if exists {
		return telemetry.NormalizedReading{EventID: raw.EventID, CraneID: raw.CraneID, SensorID: raw.SensorID, Kind: raw.Kind, Quality: telemetry.QualityDuplicate}, nil
	}
	lastSequence, err := s.repository.LastSequence(ctx, raw.SensorID)
	if err != nil {
		return telemetry.NormalizedReading{}, err
	}
	if lastSequence > 0 && raw.Sequence+s.outOfOrder < lastSequence {
		_ = s.repository.StoreQuarantined(ctx, raw, "sequence outside reorder window")
		return telemetry.NormalizedReading{}, errors.New("telemetry outside reorder window")
	}
	converted, err := s.converter.Convert(raw.Kind, raw.Value, raw.Unit, device.CanonicalUnit)
	if err != nil {
		_ = s.repository.StoreQuarantined(ctx, raw, "unit conversion failed")
		return telemetry.NormalizedReading{}, err
	}
	calibration, err := s.repository.FindCalibration(ctx, device.CalibrationID, device.CalibrationVer)
	if err != nil {
		_ = s.repository.StoreQuarantined(ctx, raw, "calibration missing")
		return telemetry.NormalizedReading{}, err
	}
	calibrated, err := calibration.Apply(converted, raw.ObservedAt)
	if err != nil || device.ValidateValue(calibrated) != nil {
		_ = s.repository.StoreQuarantined(ctx, raw, "calibrated value invalid")
		return telemetry.NormalizedReading{}, errors.New("calibrated value invalid")
	}
	quality := telemetry.QualityGood
	qualityReason := ""
	if raw.Sequence < lastSequence {
		quality = telemetry.QualityOutOfOrder
		qualityReason = "accepted within reorder window"
	}
	checksumSource := fmt.Sprintf("%s|%s|%d|%d|%s|%d", raw.EventID, raw.SensorID, calibrated, raw.Sequence, crane.SafetyConfigID, crane.SafetyConfigVer)
	sum := sha256.Sum256([]byte(checksumSource))
	normalized := telemetry.NormalizedReading{EventID: raw.EventID, CraneID: raw.CraneID, SensorID: raw.SensorID, Kind: raw.Kind, Value: calibrated, CanonicalUnit: device.CanonicalUnit, Sequence: raw.Sequence, ObservedAt: raw.ObservedAt.UTC(), ReceivedAt: now, LimitEngaged: raw.LimitEngaged, Quality: quality, QualityReason: qualityReason, CalibrationID: calibration.ID, CalibrationVer: calibration.Version, SafetyConfigID: crane.SafetyConfigID, SafetyConfigVer: crane.SafetyConfigVer, EvidenceChecksum: hex.EncodeToString(sum[:])}
	if err := s.repository.StoreReading(ctx, normalized); err != nil {
		return telemetry.NormalizedReading{}, err
	}
	return normalized, nil
}

func (s *Service) Snapshot(ctx context.Context, craneID string) (telemetry.Snapshot, error) {
	readings, err := s.repository.RecentReadings(ctx, craneID, s.clock.Now().Add(-s.staleAfter*2))
	if err != nil {
		return telemetry.Snapshot{}, err
	}
	return telemetry.NewSnapshot(craneID, s.clock.Now(), readings, s.staleAfter), nil
}

type StandardConverter struct{}

func (StandardConverter) Convert(kind sensor.Kind, value int64, from, to string) (int64, error) {
	if from == to {
		return value, nil
	}
	key := string(kind) + ":" + from + ":" + to
	switch key {
	case "load:kg:g":
		return value * 1000, nil
	case "radius:m:mm", "height:m:mm":
		return value * 1000, nil
	case "slew_angle:deg:millideg":
		return value * 1000, nil
	case "wind_speed:m/s:mm/s":
		return value * 1000, nil
	case "limit_switch:bool:bool":
		return value, nil
	default:
		return 0, errors.New("unsupported telemetry unit")
	}
}
