package memory

import (
	"context"
	"time"

	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
)

func (s *Store) SeedSensor(value sensor.Sensor) {
	s.mu.Lock()
	s.sensors[value.ID] = value
	s.mu.Unlock()
}

func (s *Store) SeedCalibration(value sensor.Calibration) {
	s.mu.Lock()
	s.calibrations[calibrationKey(value.ID, value.Version)] = value
	s.mu.Unlock()
}

func (s *Store) FindSensor(ctx context.Context, id string) (sensor.Sensor, error) {
	if err := ctx.Err(); err != nil {
		return sensor.Sensor{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.sensors[id]
	if !ok {
		return sensor.Sensor{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) FindCalibration(ctx context.Context, id string, version int64) (sensor.Calibration, error) {
	if err := ctx.Err(); err != nil {
		return sensor.Calibration{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.calibrations[calibrationKey(id, version)]
	if !ok {
		return sensor.Calibration{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) LastSequence(ctx context.Context, sensorID string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSequences[sensorID], nil
}

func (s *Store) EventExists(ctx context.Context, craneID, eventID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (s *Store) StoreReading(ctx context.Context, value telemetry.NormalizedReading) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := eventKey(value.CraneID, value.EventID)
	s.events[key] = true
	s.readings[value.CraneID] = append(s.readings[value.CraneID], value)
	if value.Sequence > s.lastSequences[value.SensorID] {
		s.lastSequences[value.SensorID] = value.Sequence
	}
	crane, ok := s.cranes[value.CraneID]
	if !ok {
		return ErrNotFound
	}
	s.timeline[crane.SiteID] = append(s.timeline[crane.SiteID], report.TimelineEvent{ID: value.EventID, CraneID: value.CraneID, Type: "telemetry", At: value.ObservedAt, Summary: string(value.Kind), Evidence: map[string]any{"checksum": value.EvidenceChecksum, "quality": value.Quality}})
	return nil
}

func (s *Store) StoreQuarantined(ctx context.Context, value telemetry.RawReading, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.quarantined = append(s.quarantined, quarantinedReading{Reading: value, Reason: reason})
	s.mu.Unlock()
	return nil
}

func (s *Store) RecentReadings(ctx context.Context, craneID string, since time.Time) ([]telemetry.NormalizedReading, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]telemetry.NormalizedReading, 0)
	for _, value := range s.readings[craneID] {
		if !value.ObservedAt.Before(since) {
			result = append(result, value)
		}
	}
	return result, nil
}

func (s *Store) LatestSnapshot(ctx context.Context, craneID string) (telemetry.Snapshot, error) {
	values, err := s.RecentReadings(ctx, craneID, time.Time{})
	if err != nil || len(values) == 0 {
		return telemetry.Snapshot{}, ErrNotFound
	}
	latest := values[0].ObservedAt
	for _, value := range values[1:] {
		if value.ObservedAt.After(latest) {
			latest = value.ObservedAt
		}
	}
	return telemetry.NewSnapshot(craneID, latest, values, 10*time.Second), nil
}

func (s *Store) QuarantinedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.quarantined)
}
