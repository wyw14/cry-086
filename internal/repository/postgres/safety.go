package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/safety"
)

func (s *Store) FindConfig(ctx context.Context, id string, version int64) (safety.Config, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM domain_objects WHERE kind='safety_config' AND id=$1 AND version=$2`, id, version).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return safety.Config{}, ErrNotFound
	}
	if err != nil {
		return safety.Config{}, err
	}
	var value safety.Config
	if err := json.Unmarshal(payload, &value); err != nil {
		return safety.Config{}, err
	}
	return value, nil
}

func (s *Store) StoreDecision(ctx context.Context, craneID string, value safety.Decision) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO safety_decisions(crane_id, level, interlock, evaluated_at, payload) VALUES($1,$2,$3,$4,$5)`, craneID, value.Level, value.Interlock, value.EvaluatedAt, payload); err != nil {
			return err
		}
		if value.Level == safety.RiskNormal {
			_, err = tx.Exec(ctx, `
				INSERT INTO safety_clearance(crane_id, clear_since) VALUES($1,$2)
				ON CONFLICT(crane_id) DO NOTHING
			`, craneID, value.EvaluatedAt)
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM safety_clearance WHERE crane_id=$1`, craneID)
		return err
	})
}

func (s *Store) LatestDecision(ctx context.Context, craneID string) (safety.Decision, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM safety_decisions WHERE crane_id=$1 ORDER BY evaluated_at DESC,id DESC LIMIT 1`, craneID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return safety.Decision{}, ErrNotFound
	}
	if err != nil {
		return safety.Decision{}, err
	}
	var value safety.Decision
	if err := json.Unmarshal(payload, &value); err != nil {
		return safety.Decision{}, err
	}
	return value, nil
}

func (s *Store) AlarmByTelemetryEvent(ctx context.Context, craneID, eventID string) (alarm.Event, bool, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM alarms WHERE crane_id=$1 AND telemetry_event_id=$2`, craneID, eventID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return alarm.Event{}, false, nil
	}
	if err != nil {
		return alarm.Event{}, false, err
	}
	var value alarm.Event
	if err := json.Unmarshal(payload, &value); err != nil {
		return alarm.Event{}, false, err
	}
	return value, true, nil
}

func (s *Store) StoreAlarm(ctx context.Context, value alarm.Event) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO alarms(id, site_id, crane_id, telemetry_event_id, status, level, interlock,
			version, evidence_hash, opened_at, payload)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, value.ID, value.SiteID, value.CraneID, value.TelemetryEventID, value.Status, value.Level, value.Interlock, value.Version, value.EvidenceHash, value.OpenedAt, payload)
	return err
}

func (s *Store) FindAlarm(ctx context.Context, id string) (alarm.Event, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM alarms WHERE id=$1`, id).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return alarm.Event{}, ErrNotFound
	}
	if err != nil {
		return alarm.Event{}, err
	}
	var value alarm.Event
	if err := json.Unmarshal(payload, &value); err != nil {
		return alarm.Event{}, err
	}
	return value, nil
}

func (s *Store) UpdateAlarm(ctx context.Context, value alarm.Event, expectedVersion int64) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE alarms SET status=$3, interlock=$4, version=$5, payload=$6, updated_at=now()
		WHERE id=$1 AND version=$2 AND evidence_hash=$7 AND telemetry_event_id=$8
	`, value.ID, expectedVersion, value.Status, value.Interlock, value.Version, payload, value.EvidenceHash, value.TelemetryEventID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("alarm version conflict or immutable evidence mismatch")
	}
	return nil
}

func (s *Store) OpenAlarms(ctx context.Context, craneID string) ([]alarm.Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT payload FROM alarms WHERE crane_id=$1 AND status <> 'resolved' ORDER BY opened_at DESC`, craneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]alarm.Event, 0)
	for rows.Next() {
		var payload []byte
		var value alarm.Event
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

func (s *Store) ConditionClearSince(ctx context.Context, craneID string) (string, error) {
	var value time.Time
	err := s.pool.QueryRow(ctx, `SELECT clear_since FROM safety_clearance WHERE crane_id=$1`, craneID).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return value.UTC().Format(time.RFC3339Nano), err
}
