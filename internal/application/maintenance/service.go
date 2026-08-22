package maintenanceapp

import (
	"context"
	"errors"

	"github.com/wyw14/cry-086/internal/domain/audit"
	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/maintenance"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	FindWorkOrder(context.Context, string) (maintenance.WorkOrder, error)
	UpdateWorkOrder(context.Context, maintenance.WorkOrder, int64) error
	StoreStopRecord(context.Context, maintenance.StopRecord) error
	FindOpenStopRecord(context.Context, string) (maintenance.StopRecord, error)
	UpdateStopRecord(context.Context, maintenance.StopRecord) error
	FindCrane(context.Context, string) (site.TowerCrane, error)
	UpdateCrane(context.Context, site.TowerCrane, int64) error
	AppendAudit(context.Context, audit.Record) error
}

type IdentityProvider interface {
	FindUser(context.Context, string) (identity.User, error)
}

type IDGenerator interface{ NewID() string }

type Service struct {
	repository Repository
	identities IdentityProvider
	clock      clock.Clock
	ids        IDGenerator
}

type resumeCommand struct {
	craneID        string
	actorID        string
	requestID      string
	expected       int64
	reviewApproved bool
}

type resumeState struct {
	crane    site.TowerCrane
	stop     maintenance.StopRecord
	operator identity.User
}

func New(repository Repository, identities IdentityProvider, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, identities: identities, clock: c, ids: ids}
}

func (s *Service) CompleteAndStop(ctx context.Context, workOrderID, actorID, requestID, result, evidenceID string, expectedVersion int64) (maintenance.WorkOrder, error) {
	order, err := s.repository.FindWorkOrder(ctx, workOrderID)
	if err != nil {
		return maintenance.WorkOrder{}, err
	}
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(order.SiteID) || !user.HasRole(identity.RoleMaintainer) {
		return maintenance.WorkOrder{}, errors.New("maintenance operation is not authorized")
	}
	if order.Version != expectedVersion {
		return maintenance.WorkOrder{}, errors.New("work order version conflict")
	}
	if err := order.Complete(result, evidenceID, s.clock.Now()); err != nil {
		return maintenance.WorkOrder{}, err
	}
	crane, err := s.repository.FindCrane(ctx, order.CraneID)
	if err != nil {
		return maintenance.WorkOrder{}, err
	}
	craneVersion := crane.Version
	if crane.Status == site.CraneRunning {
		if err := crane.Transition(site.CraneStopped, s.clock.Now()); err != nil {
			return maintenance.WorkOrder{}, err
		}
	}
	if err := s.repository.UpdateWorkOrder(ctx, order, expectedVersion); err != nil {
		return maintenance.WorkOrder{}, err
	}
	if crane.Version != craneVersion {
		if err := s.repository.UpdateCrane(ctx, crane, craneVersion); err != nil {
			return maintenance.WorkOrder{}, err
		}
	}
	stop := maintenance.StopRecord{ID: s.ids.NewID(), CraneID: crane.ID, Reason: "maintenance completion requires safety review", StoppedAt: s.clock.Now(), StoppedBy: user.ID, WorkOrderID: order.ID}
	if err := s.repository.StoreStopRecord(ctx, stop); err != nil {
		return maintenance.WorkOrder{}, err
	}
	record := audit.New(s.ids.NewID(), order.SiteID, user.ID, "api", "maintenance.completed", "work_order", order.ID, result, requestID, map[string]any{"status": maintenance.WorkInProgress}, map[string]any{"status": order.Status, "crane_status": crane.Status}, s.clock.Now(), "")
	if err := s.repository.AppendAudit(ctx, record); err != nil {
		return maintenance.WorkOrder{}, err
	}
	return order, nil
}

func (s *Service) ResumeCrane(ctx context.Context, craneID, actorID, requestID string, expectedCraneVersion int64, safetyReviewPassed bool) (site.TowerCrane, error) {
	command := resumeCommand{craneID: craneID, actorID: actorID, requestID: requestID, expected: expectedCraneVersion, reviewApproved: safetyReviewPassed}
	state, err := s.prepareResume(ctx, command)
	if err != nil {
		return site.TowerCrane{}, err
	}
	if err := s.persistResume(ctx, command, state); err != nil {
		return site.TowerCrane{}, err
	}
	if err := s.auditResume(ctx, command, state); err != nil {
		return site.TowerCrane{}, err
	}
	return state.crane, nil
}

func (s *Service) prepareResume(ctx context.Context, command resumeCommand) (resumeState, error) {
	crane, err := s.repository.FindCrane(ctx, command.craneID)
	if err != nil {
		return resumeState{}, err
	}
	operator, err := s.identities.FindUser(ctx, command.actorID)
	if err != nil || !operator.Active || !operator.OwnsSite(crane.SiteID) || !operator.HasRole(identity.RoleSafetyOfficer) {
		return resumeState{}, errors.New("resume operation is not authorized")
	}
	if crane.Version != command.expected || crane.Status != site.CraneStopped {
		return resumeState{}, errors.New("crane cannot resume from current version or state")
	}
	stop, err := s.repository.FindOpenStopRecord(ctx, command.craneID)
	if err != nil {
		return resumeState{}, err
	}
	if err := stop.Resume(operator.ID, command.reviewApproved, s.clock.Now()); err != nil {
		return resumeState{}, err
	}
	if err := crane.Transition(site.CraneRunning, s.clock.Now()); err != nil {
		return resumeState{}, err
	}
	return resumeState{crane: crane, stop: stop, operator: operator}, nil
}

func (s *Service) persistResume(ctx context.Context, command resumeCommand, state resumeState) error {
	if err := s.repository.UpdateStopRecord(ctx, state.stop); err != nil {
		return err
	}
	return s.repository.UpdateCrane(ctx, state.crane, command.expected)
}

func (s *Service) auditResume(ctx context.Context, command resumeCommand, state resumeState) error {
	record := audit.New(
		s.ids.NewID(), state.crane.SiteID, state.operator.ID, "api", "crane.resumed", "crane", state.crane.ID,
		"maintenance evidence and safety review accepted", command.requestID,
		map[string]any{"status": site.CraneStopped}, map[string]any{"status": site.CraneRunning}, s.clock.Now(), "",
	)
	return s.repository.AppendAudit(ctx, record)
}
