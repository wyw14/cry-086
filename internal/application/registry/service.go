package registry

import (
	"context"
	"errors"

	"github.com/wyw14/cry-086/internal/domain/audit"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	CreateSite(context.Context, site.Site) error
	CreateCrane(context.Context, site.TowerCrane) error
	FindSite(context.Context, string) (site.Site, error)
	FindCrane(context.Context, string) (site.TowerCrane, error)
	UpdateCrane(context.Context, site.TowerCrane, int64) error
	AppendAudit(context.Context, audit.Record) error
}

type IDGenerator interface {
	NewID() string
}

type Service struct {
	repository Repository
	clock      clock.Clock
	ids        IDGenerator
}

func New(repository Repository, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, clock: c, ids: ids}
}

func (s *Service) RegisterSite(ctx context.Context, actorID, requestID, name, timezone, ownerUnitID string, retention int) (site.Site, error) {
	if err := ctx.Err(); err != nil {
		return site.Site{}, err
	}
	created, err := site.NewSite(s.ids.NewID(), name, timezone, ownerUnitID, retention, s.clock.Now())
	if err != nil {
		return site.Site{}, err
	}
	if err := s.repository.CreateSite(ctx, created); err != nil {
		return site.Site{}, err
	}
	record := audit.New(s.ids.NewID(), created.ID, actorID, "api", "site.created", "site", created.ID, "initial registration", requestID, nil, map[string]any{"name": created.Name, "timezone": created.Timezone}, s.clock.Now(), "")
	if err := s.repository.AppendAudit(ctx, record); err != nil {
		return site.Site{}, errors.Join(errors.New("site persisted but audit failed"), err)
	}
	return created, nil
}

func (s *Service) RegisterCrane(ctx context.Context, actorID, requestID string, crane site.TowerCrane) (site.TowerCrane, error) {
	if _, err := s.repository.FindSite(ctx, crane.SiteID); err != nil {
		return site.TowerCrane{}, err
	}
	if crane.ID == "" {
		crane.ID = s.ids.NewID()
	}
	if err := s.repository.CreateCrane(ctx, crane); err != nil {
		return site.TowerCrane{}, err
	}
	record := audit.New(s.ids.NewID(), crane.SiteID, actorID, "api", "crane.registered", "crane", crane.ID, "tower crane commissioned", requestID, nil, map[string]any{"serial_number": crane.SerialNumber, "model_id": crane.ModelID}, s.clock.Now(), "")
	if err := s.repository.AppendAudit(ctx, record); err != nil {
		return site.TowerCrane{}, err
	}
	return crane, nil
}

func (s *Service) ChangeCraneStatus(ctx context.Context, actorID, requestID, craneID string, expectedVersion int64, target site.CraneStatus, reason string) (site.TowerCrane, error) {
	crane, err := s.repository.FindCrane(ctx, craneID)
	if err != nil {
		return site.TowerCrane{}, err
	}
	if crane.Version != expectedVersion {
		return site.TowerCrane{}, errors.New("crane version conflict")
	}
	before := crane.Status
	if err := crane.Transition(target, s.clock.Now()); err != nil {
		return site.TowerCrane{}, err
	}
	if err := s.repository.UpdateCrane(ctx, crane, expectedVersion); err != nil {
		return site.TowerCrane{}, err
	}
	record := audit.New(s.ids.NewID(), crane.SiteID, actorID, "api", "crane.status_changed", "crane", crane.ID, reason, requestID, map[string]any{"status": before}, map[string]any{"status": target}, s.clock.Now(), "")
	if err := s.repository.AppendAudit(ctx, record); err != nil {
		return site.TowerCrane{}, err
	}
	return crane, nil
}
