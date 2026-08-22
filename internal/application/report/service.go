package reportapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	ShiftStatistics(context.Context, report.ShiftWindow) (report.ShiftStatistics, error)
	Timeline(context.Context, string, time.Time, time.Time, int) ([]report.TimelineEvent, error)
	StoreReport(context.Context, report.RegulatoryReport) error
	ListReports(context.Context, string, string, int) ([]report.RegulatoryReport, string, error)
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

func New(repository Repository, identities IdentityProvider, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, identities: identities, clock: c, ids: ids}
}

func (s *Service) GenerateRegulatory(ctx context.Context, actorID string, window report.ShiftWindow, retentionDays int) (report.RegulatoryReport, error) {
	if !window.Valid() || retentionDays < 30 {
		return report.RegulatoryReport{}, errors.New("report window or retention is invalid")
	}
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(window.SiteID) || !user.HasRole(identity.RoleSafetyOfficer, identity.RoleRegulator) {
		return report.RegulatoryReport{}, errors.New("report generation is not authorized")
	}
	statistics, err := s.repository.ShiftStatistics(ctx, window)
	if err != nil {
		return report.RegulatoryReport{}, err
	}
	timeline, err := s.repository.Timeline(ctx, window.SiteID, window.Start, window.End, 10_000)
	if err != nil {
		return report.RegulatoryReport{}, err
	}
	sort.SliceStable(timeline, func(i, j int) bool {
		if timeline[i].At.Equal(timeline[j].At) {
			return timeline[i].ID < timeline[j].ID
		}
		return timeline[i].At.Before(timeline[j].At)
	})
	evidenceIDs := make([]string, 0, len(timeline))
	for _, event := range timeline {
		evidenceIDs = append(evidenceIDs, event.ID)
	}
	now := s.clock.Now()
	generated := report.RegulatoryReport{ID: s.ids.NewID(), SiteID: window.SiteID, Window: window, Statistics: statistics, EvidenceIDs: evidenceIDs, GeneratedAt: now, GeneratedBy: user.ID, RetentionEnd: now.AddDate(0, 0, retentionDays)}
	payload, err := json.Marshal(generated)
	if err != nil {
		return report.RegulatoryReport{}, err
	}
	hash := sha256.Sum256(payload)
	generated.ContentHash = hex.EncodeToString(hash[:])
	if err := s.repository.StoreReport(ctx, generated); err != nil {
		return report.RegulatoryReport{}, err
	}
	return generated, nil
}

func (s *Service) Replay(ctx context.Context, actorID, siteID string, start, end time.Time, limit int) ([]report.TimelineEvent, error) {
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(siteID) {
		return nil, errors.New("timeline access is not authorized")
	}
	if !start.Before(end) || end.Sub(start) > 7*24*time.Hour || limit < 1 || limit > 10_000 {
		return nil, errors.New("timeline query is invalid")
	}
	events, err := s.repository.Timeline(ctx, siteID, start.UTC(), end.UTC(), limit)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })
	return events, nil
}

func (s *Service) List(ctx context.Context, actorID, siteID, cursor string, limit int) ([]report.RegulatoryReport, string, error) {
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(siteID) || !user.HasRole(identity.RoleSafetyOfficer, identity.RoleRegulator, identity.RoleViewer) {
		return nil, "", errors.New("report listing is not authorized")
	}
	if limit < 1 || limit > 200 {
		return nil, "", errors.New("report page size is invalid")
	}
	return s.repository.ListReports(ctx, siteID, cursor, limit)
}
