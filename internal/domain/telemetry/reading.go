package telemetry

import (
	"errors"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
)

type Quality string

const (
	QualityGood       Quality = "good"
	QualityDegraded   Quality = "degraded"
	QualityInvalid    Quality = "invalid"
	QualityDuplicate  Quality = "duplicate"
	QualityOutOfOrder Quality = "out_of_order"
)

type RawReading struct {
	EventID       string      `json:"event_id" validate:"required,max=100"`
	CraneID       string      `json:"crane_id" validate:"required,max=100"`
	SensorID      string      `json:"sensor_id" validate:"required,max=100"`
	Kind          sensor.Kind `json:"kind" validate:"required"`
	Value         int64       `json:"value"`
	Unit          string      `json:"unit" validate:"required,max=20"`
	Sequence      int64       `json:"sequence" validate:"gte=0"`
	ObservedAt    time.Time   `json:"observed_at" validate:"required"`
	ReceivedAt    time.Time   `json:"received_at"`
	LimitEngaged  bool        `json:"limit_engaged"`
	SimulatorNode string      `json:"simulator_node" validate:"required,max=100"`
}

type NormalizedReading struct {
	EventID          string      `json:"event_id"`
	CraneID          string      `json:"crane_id"`
	SensorID         string      `json:"sensor_id"`
	Kind             sensor.Kind `json:"kind"`
	Value            int64       `json:"value"`
	CanonicalUnit    string      `json:"canonical_unit"`
	Sequence         int64       `json:"sequence"`
	ObservedAt       time.Time   `json:"observed_at"`
	ReceivedAt       time.Time   `json:"received_at"`
	LimitEngaged     bool        `json:"limit_engaged"`
	Quality          Quality     `json:"quality"`
	QualityReason    string      `json:"quality_reason,omitempty"`
	CalibrationID    string      `json:"calibration_id"`
	CalibrationVer   int64       `json:"calibration_version"`
	SafetyConfigID   string      `json:"safety_config_id"`
	SafetyConfigVer  int64       `json:"safety_config_version"`
	EvidenceChecksum string      `json:"evidence_checksum"`
}

func (r RawReading) Validate(now time.Time, maxFutureSkew time.Duration) error {
	if r.EventID == "" || r.CraneID == "" || r.SensorID == "" || r.Unit == "" {
		return errors.New("reading identity is incomplete")
	}
	if r.ObservedAt.IsZero() || r.ObservedAt.After(now.Add(maxFutureSkew)) {
		return errors.New("reading timestamp is invalid")
	}
	return nil
}

type Snapshot struct {
	CraneID        string                            `json:"crane_id"`
	At             time.Time                         `json:"at"`
	Readings       map[sensor.Kind]NormalizedReading `json:"readings"`
	OverallQuality Quality                           `json:"overall_quality"`
}

func NewSnapshot(craneID string, at time.Time, readings []NormalizedReading, staleAfter time.Duration) Snapshot {
	snapshot := Snapshot{CraneID: craneID, At: at.UTC(), Readings: make(map[sensor.Kind]NormalizedReading), OverallQuality: QualityGood}
	for _, reading := range readings {
		current, exists := snapshot.Readings[reading.Kind]
		if !exists || reading.ObservedAt.After(current.ObservedAt) {
			snapshot.Readings[reading.Kind] = reading
		}
		if reading.Quality != QualityGood {
			snapshot.OverallQuality = QualityDegraded
		}
	}
	mandatory := []sensor.Kind{sensor.Load, sensor.Radius, sensor.Height, sensor.SlewAngle, sensor.WindSpeed, sensor.LimitSwitch}
	for _, kind := range mandatory {
		reading, ok := snapshot.Readings[kind]
		if !ok || at.Sub(reading.ObservedAt) > staleAfter {
			snapshot.OverallQuality = QualityDegraded
		}
	}
	return snapshot
}
