package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/report"
)

func (s *Store) ShiftStatistics(ctx context.Context, window report.ShiftWindow) (report.ShiftStatistics, error) {
	stats := report.ShiftStatistics{SiteID: window.SiteID, Start: window.Start.UTC(), End: window.End.UTC()}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM domain_objects WHERE kind='crane' AND site_id=$1`, window.SiteID).Scan(&stats.CraneCount); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			SELECT count(*),
				count(*) FILTER (WHERE quality <> 'good')
			FROM telemetry_events t
			JOIN domain_objects c ON c.kind='crane' AND c.id=t.crane_id
			WHERE c.site_id=$1 AND t.observed_at >= $2 AND t.observed_at < $3 AND t.quarantined=false
		`, window.SiteID, window.Start, window.End).Scan(&stats.TelemetryCount, &stats.DegradedCount)
	})
	return stats, err
}

func (s *Store) Timeline(ctx context.Context, siteID string, start, end time.Time, limit int) ([]report.TimelineEvent, error) {
	auditRows, err := s.pool.Query(ctx, `
		SELECT id,resource_id,action,occurred_at,reason,after_data
		FROM audit_records WHERE site_id=$1 AND occurred_at >= $2 AND occurred_at < $3
		ORDER BY occurred_at,id LIMIT $4
	`, siteID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer auditRows.Close()
	values := make([]report.TimelineEvent, 0)
	for auditRows.Next() {
		var value report.TimelineEvent
		var evidenceBytes []byte
		if err := auditRows.Scan(&value.ID, &value.CraneID, &value.Type, &value.At, &value.Summary, &evidenceBytes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(evidenceBytes, &value.Evidence); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := auditRows.Err(); err != nil {
		return nil, err
	}
	telemetryRows, err := s.pool.Query(ctx, `
		SELECT t.event_id,t.crane_id,t.observed_at,t.quality,t.evidence_checksum,t.payload->>'kind'
		FROM telemetry_events t
		JOIN domain_objects c ON c.kind='crane' AND c.id=t.crane_id
		WHERE c.site_id=$1 AND t.observed_at >= $2 AND t.observed_at < $3
		ORDER BY t.observed_at,t.id LIMIT $4
	`, siteID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer telemetryRows.Close()
	for telemetryRows.Next() {
		var value report.TimelineEvent
		var quality, checksum string
		if err := telemetryRows.Scan(&value.ID, &value.CraneID, &value.At, &quality, &checksum, &value.Summary); err != nil {
			return nil, err
		}
		value.Type = "telemetry"
		value.Evidence = map[string]any{"quality": quality, "checksum": checksum}
		values = append(values, value)
	}
	if err := telemetryRows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].At.Equal(values[j].At) {
			return values[i].ID < values[j].ID
		}
		return values[i].At.Before(values[j].At)
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (s *Store) StoreReport(ctx context.Context, value report.RegulatoryReport) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO regulatory_reports(id,site_id,generated_at,content_hash,retention_end,payload)
		VALUES($1,$2,$3,$4,$5,$6)
	`, value.ID, value.SiteID, value.GeneratedAt, value.ContentHash, value.RetentionEnd, payload)
	return err
}

func (s *Store) ListReports(ctx context.Context, siteID, cursor string, limit int) ([]report.RegulatoryReport, string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM regulatory_reports
		WHERE site_id=$1 AND ($2='' OR id < $2)
		ORDER BY generated_at DESC,id DESC LIMIT $3
	`, siteID, cursor, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	values := make([]report.RegulatoryReport, 0, limit+1)
	for rows.Next() {
		var payload []byte
		var value report.RegulatoryReport
		if err := rows.Scan(&payload); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, "", err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(values) > limit {
		next = values[limit-1].ID
		values = values[:limit]
	}
	return values, next, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
