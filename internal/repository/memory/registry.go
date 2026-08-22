package memory

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/wyw14/cry-086/internal/domain/audit"
	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/domain/site"
)

func (s *Store) CreateSite(ctx context.Context, value site.Site) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.sites[value.ID]; exists {
		return errors.New("site already exists")
	}
	s.sites[value.ID] = value
	return nil
}

func (s *Store) FindSite(ctx context.Context, id string) (site.Site, error) {
	if err := ctx.Err(); err != nil {
		return site.Site{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.sites[id]
	if !ok {
		return site.Site{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) CreateCrane(ctx context.Context, value site.TowerCrane) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.cranes[value.ID]; exists {
		return errors.New("crane already exists")
	}
	for _, current := range s.cranes {
		if current.SiteID == value.SiteID && current.SerialNumber == value.SerialNumber {
			return errors.New("crane serial number already exists at site")
		}
	}
	s.cranes[value.ID] = value
	return nil
}

func (s *Store) FindCrane(ctx context.Context, id string) (site.TowerCrane, error) {
	if err := ctx.Err(); err != nil {
		return site.TowerCrane{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.cranes[id]
	if !ok {
		return site.TowerCrane{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) UpdateCrane(ctx context.Context, value site.TowerCrane, expectedVersion int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.cranes[value.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Version != expectedVersion || value.Version != expectedVersion+1 {
		return errors.New("crane optimistic version conflict")
	}
	s.cranes[value.ID] = value
	return nil
}

func (s *Store) ListCranes(ctx context.Context, siteID, cursor string, limit int, sortBy string, filters map[string]string) ([]site.TowerCrane, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	s.mu.RLock()
	values := make([]site.TowerCrane, 0)
	for _, crane := range s.cranes {
		if crane.SiteID != siteID || (cursor != "" && crane.ID <= cursor) {
			continue
		}
		if status, ok := filters["status"]; ok && string(crane.Status) != status {
			continue
		}
		if model, ok := filters["model_id"]; ok && crane.ModelID != model {
			continue
		}
		if unit, ok := filters["responsible_unit_id"]; ok && crane.ResponsibleUnitID != unit {
			continue
		}
		values = append(values, crane)
	}
	s.mu.RUnlock()
	sort.SliceStable(values, func(i, j int) bool {
		if sortBy == "serial_number" {
			return strings.Compare(values[i].SerialNumber, values[j].SerialNumber) < 0
		}
		return values[i].ID < values[j].ID
	})
	next := ""
	if len(values) > limit {
		next = values[limit-1].ID
		values = values[:limit]
	}
	return values, next, nil
}

func (s *Store) AppendAudit(ctx context.Context, record audit.Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !record.Verify() {
		return errors.New("invalid audit record hash")
	}
	s.mu.Lock()
	s.audits = append(s.audits, record)
	s.timeline[record.SiteID] = append(s.timeline[record.SiteID], report.TimelineEvent{ID: record.ID, CraneID: record.ResourceID, Type: record.Action, At: record.OccurredAt, Summary: record.Reason, Evidence: map[string]any{"audit_hash": record.Hash}})
	s.mu.Unlock()
	return nil
}

func (s *Store) Audits(siteID string) []audit.Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]audit.Record, 0)
	for _, record := range s.audits {
		if record.SiteID == siteID {
			result = append(result, record)
		}
	}
	return result
}
