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

type completionCommand struct {
	workOrderID string
	actorID     string
	requestID   string
	result      string
	evidenceID  string
	expected    int64
}

type completionState struct {
	order        maintenance.WorkOrder
	operator     identity.User
	crane        site.TowerCrane
	craneVersion int64
	stop         maintenance.StopRecord
}

func New(repository Repository, identities IdentityProvider, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, identities: identities, clock: c, ids: ids}
}

func (s *Service) CompleteAndStop(ctx context.Context, workOrderID, actorID, requestID, result, evidenceID string, expectedVersion int64) (maintenance.WorkOrder, error) {
	command := completionCommand{workOrderID: workOrderID, actorID: actorID, requestID: requestID, result: result, evidenceID: evidenceID, expected: expectedVersion}
	state, err := s.prepareCompletion(ctx, command)
	if err != nil {
		return maintenance.WorkOrder{}, err
	}
	if err := s.persistCompletion(ctx, command, &state); err != nil {
		return maintenance.WorkOrder{}, err
	}
	if err := s.auditCompletion(ctx, command, state); err != nil {
		return maintenance.WorkOrder{}, err
	}
	return state.order, nil
}

func (s *Service) prepareCompletion(ctx context.Context, command completionCommand) (completionState, error) {
	order, err := s.repository.FindWorkOrder(ctx, command.workOrderID)
	if err != nil {
		return completionState{}, err
	}
	operator, err := s.identities.FindUser(ctx, command.actorID)
	if err != nil || !operator.Active || !operator.OwnsSite(order.SiteID) || !operator.HasRole(identity.RoleMaintainer) {
		return completionState{}, errors.New("maintenance operation is not authorized")
	}
	if order.Version != command.expected {
		return completionState{}, errors.New("work order version conflict")
	}
	if err := order.Complete(command.result, command.evidenceID, s.clock.Now()); err != nil {
		return completionState{}, err
	}
	crane, err := s.repository.FindCrane(ctx, order.CraneID)
	if err != nil {
		return completionState{}, err
	}
	craneVersion := crane.Version
	if crane.Status == site.CraneRunning {
		if err := crane.Transition(site.CraneStopped, s.clock.Now()); err != nil {
			return completionState{}, err
		}
	}
	stop := maintenance.StopRecord{ID: s.ids.NewID(), CraneID: crane.ID, Reason: "maintenance completion requires safety review", StoppedAt: s.clock.Now(), StoppedBy: operator.ID, WorkOrderID: order.ID}
	return completionState{order: order, operator: operator, crane: crane, craneVersion: craneVersion, stop: stop}, nil
}

func (s *Service) persistCompletion(ctx context.Context, command completionCommand, state *completionState) error {
	if err := s.repository.UpdateWorkOrder(ctx, state.order, command.expected); err != nil {
		return err
	}
	if state.crane.Version != state.craneVersion {
		if err := s.repository.UpdateCrane(ctx, state.crane, state.craneVersion); err != nil {
			return err
		}
	}
	return s.repository.StoreStopRecord(ctx, state.stop)
}

func (s *Service) auditCompletion(ctx context.Context, command completionCommand, state completionState) error {
	record := audit.New(
		s.ids.NewID(), state.order.SiteID, state.operator.ID, "api", "maintenance.completed",
		"work_order", state.order.ID, command.result, command.requestID,
		map[string]any{"status": maintenance.WorkInProgress},
		map[string]any{"status": state.order.Status, "crane_status": state.crane.Status},
		s.clock.Now(), "",
	)
	return s.repository.AppendAudit(ctx, record)
}

func (s *Service) ResumeCrane(ctx context.Context, craneID, actorID, requestID string, expectedCraneVersion int64, safetyReviewPassed bool) (site.TowerCrane, error) {
	crane, err := s.repository.FindCrane(ctx, craneID)
	if err != nil {
		return site.TowerCrane{}, err
	}
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(crane.SiteID) || !user.HasRole(identity.RoleSafetyOfficer) {
		return site.TowerCrane{}, errors.New("resume operation is not authorized")
	}
	if crane.Version != expectedCraneVersion || crane.Status != site.CraneStopped {
		return site.TowerCrane{}, errors.New("crane cannot resume from current version or state")
	}
	stop, err := s.repository.FindOpenStopRecord(ctx, craneID)
	if err != nil {
		return site.TowerCrane{}, err
	}
	if err := stop.Resume(user.ID, safetyReviewPassed, s.clock.Now()); err != nil {
		return site.TowerCrane{}, err
	}
	if err := crane.Transition(site.CraneRunning, s.clock.Now()); err != nil {
		return site.TowerCrane{}, err
	}
	if err := s.repository.UpdateStopRecord(ctx, stop); err != nil {
		return site.TowerCrane{}, err
	}
	if err := s.repository.UpdateCrane(ctx, crane, expectedCraneVersion); err != nil {
		return site.TowerCrane{}, err
	}
	record := audit.New(s.ids.NewID(), crane.SiteID, user.ID, "api", "crane.resumed", "crane", crane.ID, "maintenance evidence and safety review accepted", requestID, map[string]any{"status": site.CraneStopped}, map[string]any{"status": site.CraneRunning}, s.clock.Now(), "")
	if err := s.repository.AppendAudit(ctx, record); err != nil {
		return site.TowerCrane{}, err
	}
	return crane, nil
}
