package alarm

import (
	"errors"
	"time"

	"github.com/wyw14/cry-086/internal/domain/safety"
)

type Status string

const (
	StatusOpen         Status = "open"
	StatusAcknowledged Status = "acknowledged"
	StatusRecovering   Status = "recovering"
	StatusResolved     Status = "resolved"
	StatusTakenOver    Status = "manual_takeover"
)

type Event struct {
	ID               string           `json:"id"`
	SiteID           string           `json:"site_id"`
	CraneID          string           `json:"crane_id"`
	TelemetryEventID string           `json:"telemetry_event_id"`
	Level            safety.RiskLevel `json:"level"`
	RuleCodes        []string         `json:"rule_codes"`
	Interlock        bool             `json:"interlock"`
	Status           Status           `json:"status"`
	OpenedAt         time.Time        `json:"opened_at"`
	AcknowledgedBy   string           `json:"acknowledged_by,omitempty"`
	AcknowledgedAt   *time.Time       `json:"acknowledged_at,omitempty"`
	RecoveryStarted  *time.Time       `json:"recovery_started_at,omitempty"`
	ResolvedBy       string           `json:"resolved_by,omitempty"`
	ResolvedAt       *time.Time       `json:"resolved_at,omitempty"`
	Version          int64            `json:"version"`
	EvidenceHash     string           `json:"evidence_hash"`
}

func New(id, siteID, craneID, telemetryEventID string, decision safety.Decision, evidenceHash string, now time.Time) (Event, error) {
	if id == "" || siteID == "" || craneID == "" || telemetryEventID == "" || evidenceHash == "" {
		return Event{}, errors.New("alarm identity is incomplete")
	}
	return Event{ID: id, SiteID: siteID, CraneID: craneID, TelemetryEventID: telemetryEventID, Level: decision.Level, RuleCodes: append([]string(nil), decision.RuleCodes...), Interlock: decision.Interlock, Status: StatusOpen, OpenedAt: now.UTC(), Version: 1, EvidenceHash: evidenceHash}, nil
}

func (e *Event) Acknowledge(actor string, at time.Time) error {
	if e.Status != StatusOpen || actor == "" {
		return errors.New("alarm cannot be acknowledged")
	}
	t := at.UTC()
	e.Status = StatusAcknowledged
	e.AcknowledgedBy = actor
	e.AcknowledgedAt = &t
	e.Version++
	return nil
}

func (e *Event) BeginRecovery(at time.Time) error {
	if e.Status != StatusAcknowledged && e.Status != StatusTakenOver {
		return errors.New("alarm is not ready for recovery")
	}
	t := at.UTC()
	e.Status = StatusRecovering
	e.RecoveryStarted = &t
	e.Version++
	return nil
}

func (e *Event) Resolve(actor string, authorized bool, conditionClearSince, at time.Time, hold time.Duration) error {
	if e.Status != StatusRecovering || !authorized || actor == "" {
		return errors.New("alarm resolution is not authorized")
	}
	if conditionClearSince.IsZero() || at.Sub(conditionClearSince) < hold {
		return errors.New("recovery hold condition not satisfied")
	}
	t := at.UTC()
	e.Status = StatusResolved
	e.ResolvedBy = actor
	e.ResolvedAt = &t
	e.Interlock = false
	e.Version++
	return nil
}

func (e *Event) TakeOver(actor string, authorized bool) error {
	if !authorized || actor == "" || e.Status == StatusResolved {
		return errors.New("manual takeover is not authorized")
	}
	e.Status = StatusTakenOver
	e.Version++
	return nil
}
