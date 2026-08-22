package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/fleet"
	"github.com/wyw14/cry-086/internal/domain/maintenance"
)

func (s *Store) Relations(ctx context.Context, siteID string) ([]fleet.Relation, error) {
	rows, err := s.pool.Query(ctx, `SELECT payload FROM fleet_relations WHERE site_id=$1 AND enabled=true ORDER BY crane_a_id,crane_b_id`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]fleet.Relation, 0)
	for rows.Next() {
		var payload []byte
		var value fleet.Relation
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

func (s *Store) LatestPose(ctx context.Context, craneID string) (fleet.Pose, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM crane_poses WHERE crane_id=$1 ORDER BY observed_at DESC LIMIT 1`, craneID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return fleet.Pose{}, ErrNotFound
	}
	if err != nil {
		return fleet.Pose{}, err
	}
	var value fleet.Pose
	if err := json.Unmarshal(payload, &value); err != nil {
		return fleet.Pose{}, err
	}
	return value, nil
}

func (s *Store) StoreCollisionRisk(ctx context.Context, value fleet.CollisionRisk) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO collision_risks(crane_a_id,crane_b_id,predicted_at,critical,payload) VALUES($1,$2,$3,$4,$5)`, value.CraneAID, value.CraneBID, value.PredictedAt, value.Critical, payload)
	return err
}

func (s *Store) FindWorkOrder(ctx context.Context, id string) (maintenance.WorkOrder, error) {
	return getObject[maintenance.WorkOrder](ctx, s.pool, "work_order", id)
}

func (s *Store) UpdateWorkOrder(ctx context.Context, value maintenance.WorkOrder, expectedVersion int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return updateObject(ctx, tx, "work_order", value.ID, expectedVersion, value.Version, value)
	})
}

func (s *Store) StoreStopRecord(ctx context.Context, value maintenance.StopRecord) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO crane_stop_records(id,crane_id,stopped_at,resumed_at,payload) VALUES($1,$2,$3,$4,$5)`, value.ID, value.CraneID, value.StoppedAt, value.ResumedAt, payload)
	return err
}

func (s *Store) FindOpenStopRecord(ctx context.Context, craneID string) (maintenance.StopRecord, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM crane_stop_records WHERE crane_id=$1 AND resumed_at IS NULL`, craneID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return maintenance.StopRecord{}, ErrNotFound
	}
	if err != nil {
		return maintenance.StopRecord{}, err
	}
	var value maintenance.StopRecord
	if err := json.Unmarshal(payload, &value); err != nil {
		return maintenance.StopRecord{}, err
	}
	return value, nil
}

func (s *Store) UpdateStopRecord(ctx context.Context, value maintenance.StopRecord) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE crane_stop_records SET resumed_at=$2,payload=$3 WHERE id=$1 AND resumed_at IS NULL`, value.ID, value.ResumedAt, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("stop record is no longer open")
	}
	return nil
}
