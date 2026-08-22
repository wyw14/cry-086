package alarmapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/audit"
	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/platform/clock"
	"github.com/wyw14/cry-086/internal/platform/notification"
)

type Repository interface {
	FindAlarm(context.Context, string) (alarm.Event, error)
	UpdateAlarm(context.Context, alarm.Event, int64) error
	AppendAudit(context.Context, audit.Record) error
	LatestDecision(context.Context, string) (safety.Decision, error)
	ConditionClearSince(context.Context, string) (timeValue string, err error)
}

type IdentityProvider interface {
	FindUser(context.Context, string) (identity.User, error)
}

type IDGenerator interface {
	NewID() string
}

type Service struct {
	repository Repository
	identities IdentityProvider
	notifier   notification.Sender
	clock      clock.Clock
	ids        IDGenerator
}

type recoveryCommand struct {
	alarmID       string
	actorID       string
	requestID     string
	reason        string
	expected      int64
	configuration safety.Config
}

type recoveryState struct {
	event      alarm.Event
	operator   identity.User
	clearSince time.Time
	before     alarm.Status
}

func New(repository Repository, identities IdentityProvider, notifier notification.Sender, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, identities: identities, notifier: notifier, clock: c, ids: ids}
}

func (s *Service) Acknowledge(ctx context.Context, alarmID, actorID, requestID string, expectedVersion int64) (alarm.Event, error) {
	event, user, err := s.authorizedEvent(ctx, alarmID, actorID, identity.RoleSafetyOfficer, identity.RoleDispatcher)
	if err != nil {
		return alarm.Event{}, err
	}
	if event.Version != expectedVersion {
		return alarm.Event{}, errors.New("alarm version conflict")
	}
	before := event.Status
	if err := event.Acknowledge(user.ID, s.clock.Now()); err != nil {
		return alarm.Event{}, err
	}
	if err := s.repository.UpdateAlarm(ctx, event, expectedVersion); err != nil {
		return alarm.Event{}, err
	}
	if err := s.appendAudit(ctx, event, user, requestID, "alarm.acknowledged", before, event.Status, "operator acknowledged the active alarm"); err != nil {
		return alarm.Event{}, err
	}
	return event, nil
}

func (s *Service) BeginRecovery(ctx context.Context, alarmID, actorID, requestID string, expectedVersion int64) (alarm.Event, error) {
	event, user, err := s.authorizedEvent(ctx, alarmID, actorID, identity.RoleSafetyOfficer)
	if err != nil {
		return alarm.Event{}, err
	}
	decision, err := s.repository.LatestDecision(ctx, event.CraneID)
	if err != nil {
		return alarm.Event{}, err
	}
	if decision.Level == safety.RiskCritical || decision.Level == safety.RiskAlarm {
		return alarm.Event{}, errors.New("high risk condition is still active")
	}
	if event.Version != expectedVersion {
		return alarm.Event{}, errors.New("alarm version conflict")
	}
	before := event.Status
	if err := event.BeginRecovery(s.clock.Now()); err != nil {
		return alarm.Event{}, err
	}
	if err := s.repository.UpdateAlarm(ctx, event, expectedVersion); err != nil {
		return alarm.Event{}, err
	}
	if err := s.appendAudit(ctx, event, user, requestID, "alarm.recovery_started", before, event.Status, "risk condition cleared and recovery hold started"); err != nil {
		return alarm.Event{}, err
	}
	return event, nil
}

func (s *Service) Resolve(ctx context.Context, alarmID, actorID, requestID, reason string, expectedVersion int64, config safety.Config) (alarm.Event, error) {
	command := recoveryCommand{alarmID: alarmID, actorID: actorID, requestID: requestID, reason: reason, expected: expectedVersion, configuration: config}
	state, err := s.prepareRecovery(ctx, command)
	if err != nil {
		return alarm.Event{}, err
	}
	if err := s.applyRecovery(ctx, command, &state); err != nil {
		return alarm.Event{}, err
	}
	if err := s.recordRecoveryEffects(ctx, command, state); err != nil {
		return alarm.Event{}, err
	}
	return state.event, nil
}

func (s *Service) prepareRecovery(ctx context.Context, command recoveryCommand) (recoveryState, error) {
	event, operator, err := s.authorizedEvent(ctx, command.alarmID, command.actorID, identity.RoleSafetyOfficer)
	if err != nil {
		return recoveryState{}, err
	}
	if event.Version != command.expected {
		return recoveryState{}, errors.New("alarm version conflict")
	}
	clearSinceText, err := s.repository.ConditionClearSince(ctx, event.CraneID)
	if err != nil {
		return recoveryState{}, err
	}
	clearSince, err := time.Parse(time.RFC3339Nano, clearSinceText)
	if err != nil {
		return recoveryState{}, errors.New("condition clear evidence is invalid")
	}
	return recoveryState{event: event, operator: operator, clearSince: clearSince, before: event.Status}, nil
}

func (s *Service) applyRecovery(ctx context.Context, command recoveryCommand, state *recoveryState) error {
	if err := state.event.Resolve(state.operator.ID, state.operator.HasRole(identity.RoleSafetyOfficer), state.clearSince, s.clock.Now(), command.configuration.RecoveryHold); err != nil {
		return err
	}
	return s.repository.UpdateAlarm(ctx, state.event, command.expected)
}

func (s *Service) recordRecoveryEffects(ctx context.Context, command recoveryCommand, state recoveryState) error {
	if err := s.appendAudit(ctx, state.event, state.operator, command.requestID, "alarm.resolved", state.before, state.event.Status, command.reason); err != nil {
		return err
	}
	notice := notification.Notice{
		ID:        s.ids.NewID(),
		SiteID:    state.event.SiteID,
		Channel:   "local_console",
		Recipient: "site:" + state.event.SiteID,
		Subject:   "Safety event resolved",
		Body:      fmt.Sprintf("Alarm %s was resolved after recovery hold", state.event.ID),
		CreatedAt: s.clock.Now(),
	}
	return s.notifier.Send(ctx, notice)
}

func (s *Service) ManualTakeover(ctx context.Context, alarmID, actorID, requestID, reason string, expectedVersion int64) (alarm.Event, error) {
	event, user, err := s.authorizedEvent(ctx, alarmID, actorID, identity.RoleSafetyOfficer, identity.RoleAdministrator)
	if err != nil {
		return alarm.Event{}, err
	}
	if event.Version != expectedVersion {
		return alarm.Event{}, errors.New("alarm version conflict")
	}
	before := event.Status
	if err := event.TakeOver(user.ID, true); err != nil {
		return alarm.Event{}, err
	}
	if err := s.repository.UpdateAlarm(ctx, event, expectedVersion); err != nil {
		return alarm.Event{}, err
	}
	if err := s.appendAudit(ctx, event, user, requestID, "alarm.manual_takeover", before, event.Status, reason); err != nil {
		return alarm.Event{}, err
	}
	return event, nil
}

func (s *Service) authorizedEvent(ctx context.Context, alarmID, actorID string, roles ...identity.Role) (alarm.Event, identity.User, error) {
	event, err := s.repository.FindAlarm(ctx, alarmID)
	if err != nil {
		return alarm.Event{}, identity.User{}, err
	}
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil {
		return alarm.Event{}, identity.User{}, err
	}
	if !user.Active || !user.OwnsSite(event.SiteID) || !user.HasRole(roles...) {
		return alarm.Event{}, identity.User{}, errors.New("alarm operation is not authorized")
	}
	return event, user, nil
}

func (s *Service) appendAudit(ctx context.Context, event alarm.Event, user identity.User, requestID, action string, before, after alarm.Status, reason string) error {
	record := audit.New(s.ids.NewID(), event.SiteID, user.ID, "api", action, "alarm", event.ID, reason, requestID, map[string]any{"status": before}, map[string]any{"status": after}, s.clock.Now(), "")
	return s.repository.AppendAudit(ctx, record)
}
