package sensor

import (
	"errors"
	"time"
)

type Kind string

const (
	Load        Kind = "load"
	Radius      Kind = "radius"
	Height      Kind = "height"
	SlewAngle   Kind = "slew_angle"
	WindSpeed   Kind = "wind_speed"
	LimitSwitch Kind = "limit_switch"
)

type CommunicationState string

const (
	CommunicationUnknown CommunicationState = "unknown"
	CommunicationOnline  CommunicationState = "online"
	CommunicationDelayed CommunicationState = "delayed"
	CommunicationOffline CommunicationState = "offline"
)

type Sensor struct {
	ID             string             `json:"id"`
	CraneID        string             `json:"crane_id"`
	Kind           Kind               `json:"kind"`
	PointCode      string             `json:"point_code"`
	NativeUnit     string             `json:"native_unit"`
	CanonicalUnit  string             `json:"canonical_unit"`
	MinValue       int64              `json:"min_value"`
	MaxValue       int64              `json:"max_value"`
	CalibrationID  string             `json:"calibration_id"`
	CalibrationVer int64              `json:"calibration_version"`
	Communication  CommunicationState `json:"communication"`
	LastSeenAt     time.Time          `json:"last_seen_at"`
	Version        int64              `json:"version"`
}

func (s Sensor) ValidateValue(value int64) error {
	if value < s.MinValue || value > s.MaxValue {
		return errors.New("sensor value outside configured range")
	}
	if s.CalibrationID == "" || s.CalibrationVer < 1 {
		return errors.New("sensor calibration unavailable")
	}
	return nil
}

func (s *Sensor) Observe(at time.Time, offlineAfter time.Duration) CommunicationState {
	at = at.UTC()
	if s.LastSeenAt.IsZero() {
		s.Communication = CommunicationUnknown
	} else if at.Sub(s.LastSeenAt) > offlineAfter {
		s.Communication = CommunicationOffline
	} else if at.Sub(s.LastSeenAt) > offlineAfter/2 {
		s.Communication = CommunicationDelayed
	} else {
		s.Communication = CommunicationOnline
	}
	return s.Communication
}

type Calibration struct {
	ID           string    `json:"id"`
	SensorID     string    `json:"sensor_id"`
	Version      int64     `json:"version"`
	OffsetMicros int64     `json:"offset_micros"`
	ScaleMicros  int64     `json:"scale_micros"`
	CertifiedBy  string    `json:"certified_by"`
	EffectiveAt  time.Time `json:"effective_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (c Calibration) Apply(raw int64, at time.Time) (int64, error) {
	if at.Before(c.EffectiveAt) || !at.Before(c.ExpiresAt) || c.ScaleMicros <= 0 {
		return 0, errors.New("calibration is not effective")
	}
	return (raw*c.ScaleMicros)/1_000_000 + c.OffsetMicros, nil
}
