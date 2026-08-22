package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/fleet"
	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/maintenance"
	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/domain/sensor"
)

func (s *Store) SeedSensor(value sensor.Sensor) {
	_ = s.withTx(context.Background(), func(tx pgx.Tx) error {
		payload, _ := json.Marshal(value)
		_, err := tx.Exec(context.Background(), `INSERT INTO domain_objects(kind,id,site_id,secondary_key,version,payload) VALUES('sensor',$1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, value.ID, value.CraneID, value.PointCode, value.Version, payload)
		return err
	})
}

func (s *Store) SeedCalibration(value sensor.Calibration) {
	_ = s.withTx(context.Background(), func(tx pgx.Tx) error {
		payload, _ := json.Marshal(value)
		_, err := tx.Exec(context.Background(), `INSERT INTO domain_objects(kind,id,site_id,secondary_key,version,payload) VALUES('calibration',$1,'',$2,$3,$4) ON CONFLICT DO NOTHING`, value.ID, value.SensorID, value.Version, payload)
		return err
	})
}

func (s *Store) SeedConfig(value safety.Config) {
	_ = s.withTx(context.Background(), func(tx pgx.Tx) error {
		payload, _ := json.Marshal(value)
		_, err := tx.Exec(context.Background(), `INSERT INTO domain_objects(kind,id,site_id,secondary_key,version,payload) VALUES('safety_config',$1,'',$2,$3,$4) ON CONFLICT DO NOTHING`, value.ID, value.CraneModelID, value.Version, payload)
		return err
	})
}

func (s *Store) SeedUser(value identity.User) {
	_ = s.withTx(context.Background(), func(tx pgx.Tx) error {
		payload, _ := json.Marshal(value)
		_, err := tx.Exec(context.Background(), `INSERT INTO domain_objects(kind,id,site_id,secondary_key,version,payload) VALUES('user',$1,'',$2,$3,$4) ON CONFLICT DO NOTHING`, value.ID, value.Username, value.Version, payload)
		return err
	})
}

func (s *Store) SeedRelation(siteID string, value fleet.Relation) {
	payload, _ := json.Marshal(value)
	_, _ = s.pool.Exec(context.Background(), `INSERT INTO fleet_relations(site_id,crane_a_id,crane_b_id,enabled,version,payload) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, siteID, value.CraneAID, value.CraneBID, value.Enabled, value.Version, payload)
}

func (s *Store) SeedPose(value fleet.Pose) {
	payload, _ := json.Marshal(value)
	_, _ = s.pool.Exec(context.Background(), `INSERT INTO crane_poses(crane_id,observed_at,payload) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, value.CraneID, value.ObservedAt, payload)
}

func (s *Store) SeedWorkOrder(value maintenance.WorkOrder) {
	_ = s.withTx(context.Background(), func(tx pgx.Tx) error {
		payload, _ := json.Marshal(value)
		_, err := tx.Exec(context.Background(), `INSERT INTO domain_objects(kind,id,site_id,secondary_key,version,payload) VALUES('work_order',$1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, value.ID, value.SiteID, value.CraneID, value.Version, payload)
		return err
	})
}
