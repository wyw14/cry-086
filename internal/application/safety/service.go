package safetyapp

import (
	"context"
	"errors"
	"time"

	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	FindCrane(context.Context, string) (site.TowerCrane, error)
	FindConfig(context.Context, string, int64) (safety.Config, error)
	AlarmByTelemetryEvent(context.Context, string, string) (alarm.Event, bool, error)
	StoreAlarm(context.Context, alarm.Event) error
	StoreDecision(context.Context, string, safety.Decision) error
}

type SnapshotProvider interface {
	Snapshot(context.Context, string) (telemetry.Snapshot, error)
}

type IDGenerator interface {
	NewID() string
}

type Service struct {
	repository Repository
	snapshots  SnapshotProvider
	clock      clock.Clock
	ids        IDGenerator
}

type evaluationContext struct {
	crane         site.TowerCrane
	configuration safety.Config
	snapshot      telemetry.Snapshot
	decision      safety.Decision
	triggerID     string
	evidenceHash  string
}

func New(repository Repository, snapshots SnapshotProvider, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, snapshots: snapshots, clock: c, ids: ids}
}

func (s *Service) Evaluate(ctx context.Context, craneID, triggerEventID, evidenceHash string) (safety.Decision, *alarm.Event, error) {
	state := evaluationContext{triggerID: triggerEventID, evidenceHash: evidenceHash}
	if err := s.loadEvaluationState(ctx, craneID, &state); err != nil {
		return safety.Decision{}, nil, err
	}
	if state.decision.Level == safety.RiskDegraded && state.configuration.ID == "" {
		return state.decision, nil, nil
	}
	if err := s.repository.StoreDecision(ctx, craneID, state.decision); err != nil {
		return safety.Decision{}, nil, err
	}
	return s.raiseAlarm(ctx, &state)
}

func (s *Service) loadEvaluationState(ctx context.Context, craneID string, state *evaluationContext) error {
	crane, err := s.repository.FindCrane(ctx, craneID)
	if err != nil {
		return err
	}
	state.crane = crane
	configuration, err := s.repository.FindConfig(ctx, crane.SafetyConfigID, crane.SafetyConfigVer)
	if err != nil {
		state.decision = safety.Decision{Level: safety.RiskDegraded, RuleCodes: []string{"CONFIGURATION_UNAVAILABLE"}, Message: "current safety configuration cannot be confirmed", ConfigID: crane.SafetyConfigID, ConfigVersion: crane.SafetyConfigVer, EvaluatedAt: s.clock.Now()}
		_ = s.repository.StoreDecision(ctx, craneID, state.decision)
		return nil
	}
	if configuration.CraneModelID != crane.ModelID {
		return errors.New("safety configuration model mismatch")
	}
	snapshot, err := s.snapshots.Snapshot(ctx, craneID)
	if err != nil {
		return err
	}
	state.configuration = configuration
	state.snapshot = snapshot
	state.decision = safety.Evaluate(configuration, snapshot, s.clock.Now())
	return nil
}

func (s *Service) raiseAlarm(ctx context.Context, state *evaluationContext) (safety.Decision, *alarm.Event, error) {
	if state.decision.Level == safety.RiskNormal || state.decision.Level == safety.RiskDegraded {
		return state.decision, nil, nil
	}
	existing, found, err := s.repository.AlarmByTelemetryEvent(ctx, state.crane.ID, state.triggerID)
	if err != nil {
		return safety.Decision{}, nil, err
	}
	if found {
		return state.decision, &existing, nil
	}
	created, err := alarm.New(s.ids.NewID(), state.crane.SiteID, state.crane.ID, state.triggerID, state.decision, state.evidenceHash, s.clock.Now())
	if err != nil {
		return safety.Decision{}, nil, err
	}
	if err := s.repository.StoreAlarm(ctx, created); err != nil {
		return safety.Decision{}, nil, err
	}
	return state.decision, &created, nil
}

func (s *Service) EvaluateAfter(ctx context.Context, craneID string, delay time.Duration) (safety.Decision, error) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return safety.Decision{}, ctx.Err()
	case <-timer.C:
		decision, _, err := s.Evaluate(ctx, craneID, "scheduled:"+s.clock.Now().Format(time.RFC3339Nano), "scheduled")
		return decision, err
	}
}
