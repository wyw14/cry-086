package reportapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/report"
	platformclock "github.com/wyw14/cry-086/internal/platform/clock"
	"github.com/wyw14/cry-086/internal/repository/memory"
)

type reportIDs struct{ next int }

func (i *reportIDs) NewID() string { i.next++; return fmt.Sprintf("report-%d", i.next) }

func TestReportListExcludesExpiredRetentionRecords(t *testing.T) {
	now := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	repository := memory.New()
	repository.SeedUser(identity.User{ID: "regulator-l", SiteIDs: []string{"site-l"}, Roles: []identity.Role{identity.RoleRegulator}, Active: true})
	expired := report.RegulatoryReport{ID: "report-expired", SiteID: "site-l", GeneratedAt: now.AddDate(-1, 0, 0), RetentionEnd: now.Add(-time.Second), ContentHash: "expired-hash"}
	active := report.RegulatoryReport{ID: "report-active", SiteID: "site-l", GeneratedAt: now.Add(-time.Hour), RetentionEnd: now.AddDate(0, 1, 0), ContentHash: "active-hash"}
	if err := repository.StoreReport(context.Background(), expired); err != nil {
		t.Fatal(err)
	}
	if err := repository.StoreReport(context.Background(), active); err != nil {
		t.Fatal(err)
	}
	service := New(repository, repository, platformclock.NewFixed(now), &reportIDs{})
	items, next, err := service.List(context.Background(), "regulator-l", "site-l", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != active.ID || next != "" {
		t.Fatalf("retained reports=%+v next=%q", items, next)
	}
}
