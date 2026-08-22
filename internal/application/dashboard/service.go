package dashboard

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/wyw14/cry-086/internal/domain/alarm"
	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
	"github.com/wyw14/cry-086/internal/platform/clock"
)

type Repository interface {
	ListCranes(context.Context, string, string, int, string, map[string]string) ([]site.TowerCrane, string, error)
	LatestSnapshot(context.Context, string) (telemetry.Snapshot, error)
	LatestDecision(context.Context, string) (safety.Decision, error)
	OpenAlarms(context.Context, string) ([]alarm.Event, error)
}

type IdentityProvider interface {
	FindUser(context.Context, string) (identity.User, error)
}

type CraneCard struct {
	Crane      site.TowerCrane    `json:"crane"`
	Snapshot   telemetry.Snapshot `json:"snapshot"`
	Decision   safety.Decision    `json:"decision"`
	OpenAlarms int                `json:"open_alarms"`
	DataAge    time.Duration      `json:"data_age"`
}

type View struct {
	SiteID      string      `json:"site_id"`
	GeneratedAt time.Time   `json:"generated_at"`
	Cards       []CraneCard `json:"cards"`
	NextCursor  string      `json:"next_cursor,omitempty"`
}

type Service struct {
	repository Repository
	identities IdentityProvider
	clock      clock.Clock
}

func New(repository Repository, identities IdentityProvider, c clock.Clock) *Service {
	return &Service{repository: repository, identities: identities, clock: c}
}

func (s *Service) Load(ctx context.Context, actorID, siteID, cursor string, limit int, sortBy string, filters map[string]string) (View, error) {
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(siteID) {
		return View{}, errors.New("dashboard access is not authorized")
	}
	allowedSort := map[string]bool{"serial_number": true, "risk": true, "last_seen": true}
	if !allowedSort[sortBy] || limit < 1 || limit > 100 {
		return View{}, errors.New("dashboard query is invalid")
	}
	allowedFilter := map[string]bool{"status": true, "model_id": true, "responsible_unit_id": true}
	for key := range filters {
		if !allowedFilter[key] {
			return View{}, errors.New("dashboard filter is not allowed")
		}
	}
	cranes, next, err := s.repository.ListCranes(ctx, siteID, cursor, limit, sortBy, filters)
	if err != nil {
		return View{}, err
	}
	cards := make([]CraneCard, 0, len(cranes))
	for _, crane := range cranes {
		snapshot, snapshotErr := s.repository.LatestSnapshot(ctx, crane.ID)
		if snapshotErr != nil {
			snapshot = telemetry.Snapshot{CraneID: crane.ID, At: s.clock.Now(), OverallQuality: telemetry.QualityDegraded}
		}
		decision, decisionErr := s.repository.LatestDecision(ctx, crane.ID)
		if decisionErr != nil {
			decision = safety.Decision{Level: safety.RiskDegraded, Message: "current decision unavailable"}
		}
		alarms, alarmErr := s.repository.OpenAlarms(ctx, crane.ID)
		if alarmErr != nil {
			return View{}, alarmErr
		}
		cards = append(cards, CraneCard{Crane: crane, Snapshot: snapshot, Decision: decision, OpenAlarms: len(alarms), DataAge: s.clock.Now().Sub(snapshot.At)})
	}
	if sortBy == "risk" {
		rank := map[safety.RiskLevel]int{safety.RiskCritical: 5, safety.RiskAlarm: 4, safety.RiskWarning: 3, safety.RiskDegraded: 2, safety.RiskNormal: 1}
		sort.SliceStable(cards, func(i, j int) bool { return rank[cards[i].Decision.Level] > rank[cards[j].Decision.Level] })
	}
	return View{SiteID: siteID, GeneratedAt: s.clock.Now(), Cards: cards, NextCursor: next}, nil
}
