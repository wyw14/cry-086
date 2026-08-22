package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/audit"
	"github.com/wyw14/cry-086/internal/domain/site"
)

func (s *Store) CreateSite(ctx context.Context, value site.Site) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return putObject(ctx, tx, "site", value.ID, value.ID, value.Name, value.Version, value)
	})
}

func (s *Store) FindSite(ctx context.Context, id string) (site.Site, error) {
	return getObject[site.Site](ctx, s.pool, "site", id)
}

func (s *Store) CreateCrane(ctx context.Context, value site.TowerCrane) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var siteExists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domain_objects WHERE kind='site' AND id=$1)`, value.SiteID).Scan(&siteExists); err != nil {
			return err
		}
		if !siteExists {
			return errors.New("site does not exist")
		}
		return putObject(ctx, tx, "crane", value.ID, value.SiteID, value.SerialNumber, value.Version, value)
	})
}

func (s *Store) FindCrane(ctx context.Context, id string) (site.TowerCrane, error) {
	return getObject[site.TowerCrane](ctx, s.pool, "crane", id)
}

func (s *Store) UpdateCrane(ctx context.Context, value site.TowerCrane, expectedVersion int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return updateObject(ctx, tx, "crane", value.ID, expectedVersion, value.Version, value)
	})
}

func (s *Store) ListCranes(ctx context.Context, siteID, cursor string, limit int, sortBy string, filters map[string]string) ([]site.TowerCrane, string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM domain_objects
		WHERE kind='crane' AND site_id=$1 AND ($2='' OR id>$2)
		ORDER BY id LIMIT $3
	`, siteID, cursor, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	values := make([]site.TowerCrane, 0, limit+1)
	for rows.Next() {
		var payload []byte
		var value site.TowerCrane
		if err := rows.Scan(&payload); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, "", err
		}
		if status, ok := filters["status"]; ok && string(value.Status) != status {
			continue
		}
		if model, ok := filters["model_id"]; ok && value.ModelID != model {
			continue
		}
		if unit, ok := filters["responsible_unit_id"]; ok && value.ResponsibleUnitID != unit {
			continue
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if sortBy == "serial_number" {
		sort.SliceStable(values, func(i, j int) bool { return strings.Compare(values[i].SerialNumber, values[j].SerialNumber) < 0 })
	}
	next := ""
	if len(values) > limit {
		next = values[limit-1].ID
		values = values[:limit]
	}
	return values, next, nil
}

func (s *Store) AppendAudit(ctx context.Context, record audit.Record) error {
	if !record.Verify() {
		return errors.New("invalid audit hash")
	}
	before, err := json.Marshal(record.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(record.After)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_records(id, site_id, actor_id, source, action, resource, resource_id,
			before_data, after_data, reason, request_id, occurred_at, previous_hash, record_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),$14)
	`, record.ID, record.SiteID, record.ActorID, record.Source, record.Action, record.Resource, record.ResourceID, before, after, record.Reason, record.RequestID, record.OccurredAt, record.PreviousHash, record.Hash)
	return err
}
