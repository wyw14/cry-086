package memory

import (
	"context"
	"errors"
	"time"

	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/domain/safety"
)

func (s *Store) SeedConfig(value safety.Config) {
	s.mu.Lock()
	s.configs[configKey(value.ID, value.Version)] = value
	s.mu.Unlock()
}

func (s *Store) FindConfig(ctx context.Context, id string, version int64) (safety.Config, error) {
	if err := ctx.Err(); err != nil {
		return safety.Config{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var value safety.Config
	found := false
	for _, candidate := range s.configs {
		if candidate.ID == id && (!found || candidate.Version > value.Version) {
			value = candidate
			found = true
		}
	}
	if !found {
		return safety.Config{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) StoreDecision(ctx context.Context, craneID string, value safety.Decision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.decisions[craneID] = append(s.decisions[craneID], value)
	if value.Level == safety.RiskNormal {
		if _, exists := s.clearSince[craneID]; !exists {
			s.clearSince[craneID] = value.EvaluatedAt
		}
	} else {
		delete(s.clearSince, craneID)
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) LatestDecision(ctx context.Context, craneID string) (safety.Decision, error) {
	if err := ctx.Err(); err != nil {
		return safety.Decision{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := s.decisions[craneID]
	if len(values) == 0 {
		return safety.Decision{}, ErrNotFound
	}
	return values[len(values)-1], nil
}

func (s *Store) AlarmByTelemetryEvent(ctx context.Context, craneID, eventID string) (alarm.Event, bool, error) {
	if err := ctx.Err(); err != nil {
		return alarm.Event{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.alarmByEvent[eventKey(craneID, eventID)]
	if !ok {
		return alarm.Event{}, false, nil
	}
	value, ok := s.alarms[id]
	return value, ok, nil
}

func (s *Store) StoreAlarm(ctx context.Context, value alarm.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := eventKey(value.CraneID, value.TelemetryEventID)
	if _, exists := s.alarmByEvent[key]; exists {
		return errors.New("alarm already exists for telemetry event")
	}
	s.alarms[value.ID] = value
	s.alarmByEvent[key] = value.ID
	s.timeline[value.SiteID] = append(s.timeline[value.SiteID], report.TimelineEvent{ID: value.ID, CraneID: value.CraneID, Type: "alarm.opened", At: value.OpenedAt, Summary: string(value.Level), Evidence: map[string]any{"evidence_hash": value.EvidenceHash, "interlock": value.Interlock}})
	return nil
}

func (s *Store) FindAlarm(ctx context.Context, id string) (alarm.Event, error) {
	if err := ctx.Err(); err != nil {
		return alarm.Event{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.alarms[id]
	if !ok {
		return alarm.Event{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) UpdateAlarm(ctx context.Context, value alarm.Event, expectedVersion int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.alarms[value.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Version != expectedVersion || value.Version != expectedVersion+1 {
		return errors.New("alarm optimistic version conflict")
	}
	if current.EvidenceHash != value.EvidenceHash || current.TelemetryEventID != value.TelemetryEventID || !current.OpenedAt.Equal(value.OpenedAt) {
		return errors.New("alarm evidence is immutable")
	}
	s.alarms[value.ID] = value
	return nil
}

func (s *Store) OpenAlarms(ctx context.Context, craneID string) ([]alarm.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]alarm.Event, 0)
	for _, value := range s.alarms {
		if value.CraneID == craneID && value.Status != alarm.StatusResolved {
			values = append(values, value)
		}
	}
	return values, nil
}

func (s *Store) ConditionClearSince(ctx context.Context, craneID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.clearSince[craneID]
	if !ok {
		return "", ErrNotFound
	}
	return value.UTC().Format(time.RFC3339Nano), nil
}
