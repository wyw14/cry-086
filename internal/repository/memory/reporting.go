package memory

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/domain/safety"
)

func (s *Store) ShiftStatistics(ctx context.Context, window report.ShiftWindow) (report.ShiftStatistics, error) {
	if err := ctx.Err(); err != nil {
		return report.ShiftStatistics{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := report.ShiftStatistics{SiteID: window.SiteID, Start: window.Start.UTC(), End: window.End.UTC()}
	cranes := make(map[string]bool)
	for _, crane := range s.cranes {
		if crane.SiteID == window.SiteID {
			cranes[crane.ID] = true
		}
	}
	stats.CraneCount = len(cranes)
	acknowledgements := make([]time.Duration, 0)
	for craneID := range cranes {
		for _, reading := range s.readings[craneID] {
			if !reading.ObservedAt.Before(window.Start) && reading.ObservedAt.Before(window.End) {
				stats.TelemetryCount++
				if reading.Quality != "good" {
					stats.DegradedCount++
				}
			}
		}
		for _, decision := range s.decisions[craneID] {
			if decision.EvaluatedAt.Before(window.Start) || !decision.EvaluatedAt.Before(window.End) {
				continue
			}
			switch decision.Level {
			case safety.RiskWarning:
				stats.WarningCount++
			case safety.RiskAlarm, safety.RiskCritical:
				stats.AlarmCount++
			}
			if decision.Interlock {
				stats.InterlockCount++
			}
		}
	}
	for _, event := range s.alarms {
		if !cranes[event.CraneID] || event.AcknowledgedAt == nil || event.OpenedAt.Before(window.Start) || !event.OpenedAt.Before(window.End) {
			continue
		}
		acknowledgements = append(acknowledgements, event.AcknowledgedAt.Sub(event.OpenedAt))
	}
	if len(acknowledgements) > 0 {
		sort.Slice(acknowledgements, func(i, j int) bool { return acknowledgements[i] < acknowledgements[j] })
		index := (len(acknowledgements)*95 + 99) / 100
		stats.AcknowledgementP95 = acknowledgements[index-1]
	}
	return stats, nil
}

func (s *Store) Timeline(ctx context.Context, siteID string, start, end time.Time, limit int) ([]report.TimelineEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	values := append([]report.TimelineEvent(nil), s.timeline[siteID]...)
	for index := range values {
		if values[index].Evidence == nil {
			values[index].Evidence = make(map[string]any)
		}
		reads, _ := values[index].Evidence["replay_reads"].(int)
		values[index].Evidence["replay_reads"] = reads + 1
	}
	s.mu.Unlock()
	result := make([]report.TimelineEvent, 0)
	for _, value := range values {
		if !value.At.Before(start) && value.At.Before(end) {
			result = append(result, value)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].At.Before(result[j].At) })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) StoreReport(ctx context.Context, value report.RegulatoryReport) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, current := range s.reports[value.SiteID] {
		if current.ID == value.ID || current.ContentHash == value.ContentHash {
			return errors.New("regulatory report already exists")
		}
	}
	s.reports[value.SiteID] = append(s.reports[value.SiteID], value)
	return nil
}

func (s *Store) ListReports(ctx context.Context, siteID, cursor string, limit int) ([]report.RegulatoryReport, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	s.mu.RLock()
	values := append([]report.RegulatoryReport(nil), s.reports[siteID]...)
	s.mu.RUnlock()
	sort.SliceStable(values, func(i, j int) bool { return values[i].GeneratedAt.After(values[j].GeneratedAt) })
	start := 0
	if cursor != "" {
		for index, value := range values {
			if value.ID == cursor {
				start = index + 1
				break
			}
		}
	}
	values = values[start:]
	next := ""
	if len(values) > limit {
		next = values[limit-1].ID
		values = values[:limit]
	}
	return values, next, nil
}

func (s *Store) AlarmCount(status alarm.Status) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, value := range s.alarms {
		if value.Status == status {
			count++
		}
	}
	return count
}
