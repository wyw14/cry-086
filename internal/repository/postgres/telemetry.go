package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
)

func (s *Store) FindSensor(ctx context.Context, id string) (sensor.Sensor, error) {
	return getObject[sensor.Sensor](ctx, s.pool, "sensor", id)
}

func (s *Store) FindCalibration(ctx context.Context, id string, version int64) (sensor.Calibration, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM domain_objects WHERE kind='calibration' AND id=$1 AND version=$2`, id, version).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return sensor.Calibration{}, ErrNotFound
	}
	if err != nil {
		return sensor.Calibration{}, err
	}
	var value sensor.Calibration
	if err := json.Unmarshal(payload, &value); err != nil {
		return sensor.Calibration{}, err
	}
	return value, nil
}

func (s *Store) LastSequence(ctx context.Context, sensorID string) (int64, error) {
	var sequence int64
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(sequence),0) FROM telemetry_events WHERE sensor_id=$1 AND quarantined=false`, sensorID).Scan(&sequence)
	return sequence, err
}

func (s *Store) EventExists(ctx context.Context, craneID, eventID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_events WHERE crane_id=$1 AND event_id=$2)`, craneID, eventID).Scan(&exists)
	return exists, err
}

func (s *Store) StoreReading(ctx context.Context, value telemetry.NormalizedReading) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO telemetry_events(event_id, crane_id, sensor_id, sequence, observed_at, received_at,
			quality, evidence_checksum, quarantined, payload)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,false,$9)
	`, value.EventID, value.CraneID, value.SensorID, value.Sequence, value.ObservedAt, value.ReceivedAt, value.Quality, value.EvidenceChecksum, payload)
	return err
}

func (s *Store) StoreQuarantined(ctx context.Context, value telemetry.RawReading, reason string) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO telemetry_events(event_id, crane_id, sensor_id, sequence, observed_at, received_at,
			quality, evidence_checksum, quarantined, quarantine_reason, payload)
		VALUES($1,$2,$3,$4,$5,$6,'invalid','',true,$7,$8)
		ON CONFLICT(crane_id,event_id) DO NOTHING
	`, value.EventID, value.CraneID, value.SensorID, value.Sequence, value.ObservedAt, value.ReceivedAt, reason, payload)
	return err
}

func (s *Store) RecentReadings(ctx context.Context, craneID string, since time.Time) ([]telemetry.NormalizedReading, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM telemetry_events
		WHERE crane_id=$1 AND observed_at >= $2 AND quarantined=false
		ORDER BY observed_at
	`, craneID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]telemetry.NormalizedReading, 0)
	for rows.Next() {
		var payload []byte
		var value telemetry.NormalizedReading
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) LatestSnapshot(ctx context.Context, craneID string) (telemetry.Snapshot, error) {
	values, err := s.RecentReadings(ctx, craneID, time.Now().UTC().Add(-time.Hour))
	if err != nil || len(values) == 0 {
		return telemetry.Snapshot{}, ErrNotFound
	}
	latest := values[len(values)-1].ObservedAt
	return telemetry.NewSnapshot(craneID, latest, values, 10*time.Second), nil
}
