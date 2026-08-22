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
	event, user, err := s.authorizedEvent(ctx, alarmID, actorID, identity.RoleSafetyOfficer)
	if err != nil {
		return alarm.Event{}, err
	}
	if event.Version != expectedVersion {
		return alarm.Event{}, errors.New("alarm version conflict")
	}
	clearSinceText, err := s.repository.ConditionClearSince(ctx, event.CraneID)
	if err != nil {
		return alarm.Event{}, err
	}
	clearSince, err := time.Parse(time.RFC3339Nano, clearSinceText)
	if err != nil {
		return alarm.Event{}, errors.New("condition clear evidence is invalid")
	}
	before := event.Status
	if err := event.Resolve(user.ID, user.HasRole(identity.RoleSafetyOfficer), clearSince, s.clock.Now(), config.RecoveryHold); err != nil {
		return alarm.Event{}, err
	}
	if err := s.repository.UpdateAlarm(ctx, event, expectedVersion); err != nil {
		return alarm.Event{}, err
	}
	if err := s.appendAudit(ctx, event, user, requestID, "alarm.resolved", before, event.Status, reason); err != nil {
		return alarm.Event{}, err
	}
	notice := notification.Notice{ID: s.ids.NewID(), SiteID: event.SiteID, Channel: "local_console", Recipient: "site:" + event.SiteID, Subject: "Safety event resolved", Body: fmt.Sprintf("Alarm %s was resolved after recovery hold", event.ID), CreatedAt: s.clock.Now()}
	if err := s.notifier.Send(ctx, notice); err != nil {
		return alarm.Event{}, err
	}
	return event, nil
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
