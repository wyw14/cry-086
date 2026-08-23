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

type ingestionState struct {
	raw          telemetry.RawReading
	crane        site.TowerCrane
	sensor       sensor.Sensor
	calibration  sensor.Calibration
	lastSequence int64
	value        int64
	quality      telemetry.Quality
	reason       string
}

func New(repository Repository, converter UnitConverter, c clock.Clock) *Service {
	return &Service{repository: repository, converter: converter, clock: c, maxFuture: 5 * time.Second, outOfOrder: 3, staleAfter: 10 * time.Second}
}

func (s *Service) Ingest(ctx context.Context, raw telemetry.RawReading) (telemetry.NormalizedReading, error) {
	now := s.clock.Now()
	raw.ReceivedAt = now
	state := ingestionState{raw: raw, quality: telemetry.QualityGood}
	if err := s.inspectIdentity(ctx, &state, now); err != nil {
		return telemetry.NormalizedReading{}, err
	}
	exists, err := s.repository.EventExists(ctx, raw.CraneID, raw.EventID)
	if err != nil {
		return telemetry.NormalizedReading{}, err
	}
	if exists {
		return telemetry.NormalizedReading{EventID: raw.EventID, CraneID: raw.CraneID, SensorID: raw.SensorID, Kind: raw.Kind, Quality: telemetry.QualityDuplicate}, nil
	}
	if err := s.inspectSequence(ctx, &state); err != nil {
		return telemetry.NormalizedReading{}, err
	}
	if err := s.normalizeValue(ctx, &state); err != nil {
		return telemetry.NormalizedReading{}, err
	}
	checksumSource := fmt.Sprintf("%s|%s|%d|%d|%s|%d", raw.EventID, raw.SensorID, state.value, raw.Sequence, state.crane.SafetyConfigID, state.crane.SafetyConfigVer)
	sum := sha256.Sum256([]byte(checksumSource))
	normalized := telemetry.NormalizedReading{EventID: raw.EventID, CraneID: raw.CraneID, SensorID: raw.SensorID, Kind: raw.Kind, Value: state.value, CanonicalUnit: state.sensor.CanonicalUnit, Sequence: raw.Sequence, ObservedAt: raw.ObservedAt.UTC(), ReceivedAt: now, LimitEngaged: raw.LimitEngaged, Quality: state.quality, QualityReason: state.reason, CalibrationID: state.calibration.ID, CalibrationVer: state.calibration.Version, SafetyConfigID: state.crane.SafetyConfigID, SafetyConfigVer: state.crane.SafetyConfigVer, EvidenceChecksum: hex.EncodeToString(sum[:])}
	if err := s.repository.StoreReading(ctx, normalized); err != nil {
		return telemetry.NormalizedReading{}, err
	}
	return normalized, nil
}

func (s *Service) inspectIdentity(ctx context.Context, state *ingestionState, now time.Time) error {
	if err := state.raw.Validate(now, s.maxFuture); err != nil {
		_ = s.repository.StoreQuarantined(ctx, state.raw, err.Error())
		return err
	}
	crane, err := s.repository.FindCrane(ctx, state.raw.CraneID)
	if err != nil {
		return err
	}
	device, err := s.repository.FindSensor(ctx, state.raw.SensorID)
	if err != nil || device.CraneID != state.raw.CraneID || device.Kind != state.raw.Kind {
		_ = s.repository.StoreQuarantined(ctx, state.raw, "sensor binding mismatch")
		return errors.New("sensor binding mismatch")
	}
	state.crane = crane
	state.sensor = device
	return nil
}

func (s *Service) inspectSequence(ctx context.Context, state *ingestionState) error {
	lastSequence, err := s.repository.LastSequence(ctx, state.raw.SensorID)
	if err != nil {
		return err
	}
	state.lastSequence = lastSequence
	if lastSequence > 0 {
		if state.raw.Sequence > lastSequence+s.outOfOrder || state.raw.Sequence < lastSequence-s.outOfOrder {
			_ = s.repository.StoreQuarantined(ctx, state.raw, "sequence outside reorder window")
			return errors.New("telemetry outside reorder window")
		}
		if state.raw.Sequence < lastSequence {
			state.quality = telemetry.QualityOutOfOrder
			state.reason = "accepted within reorder window"
		}
	}
	return nil
}

func (s *Service) normalizeValue(ctx context.Context, state *ingestionState) error {
	converted, err := s.converter.Convert(state.raw.Kind, state.raw.Value, state.raw.Unit, state.sensor.CanonicalUnit)
	if err != nil {
		_ = s.repository.StoreQuarantined(ctx, state.raw, "unit conversion failed")
		return err
	}
	calibration, err := s.repository.FindCalibration(ctx, state.sensor.CalibrationID, state.sensor.CalibrationVer)
	if err != nil {
		_ = s.repository.StoreQuarantined(ctx, state.raw, "calibration missing")
		return err
	}
	calibrated, err := calibration.Apply(converted, state.raw.ObservedAt)
	if err != nil || state.sensor.ValidateValue(calibrated) != nil {
		_ = s.repository.StoreQuarantined(ctx, state.raw, "calibrated value invalid")
		return errors.New("calibrated value invalid")
	}
	state.calibration = calibration
	state.value = calibrated
	return nil
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
