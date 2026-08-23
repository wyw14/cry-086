package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/repository/postgres"
)

// TestPostgresFindConfigHonorsBoundVersion guards against regressing to
// "latest version wins" semantics in the PostgreSQL store. A crane pinned to an
// older safety configuration version must resolve that exact version even when a
// newer one is staged, so safety rules are evaluated against the curve the
// device is actually bound to.
func TestPostgresFindConfigHonorsBoundVersion(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	stamp := time.Now().UTC().Format("150405.000000000")
	configID := "integration-config-" + stamp
	base := safety.Config{
		ID:                  configID,
		CraneModelID:        "model-zt6015",
		EffectiveAt:         time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC),
		WarningWindMilliMPS: 12_000,
		StopWindMilliMPS:    15_000,
		RecoveryHold:        30 * time.Second,
		CombinationMargin:   10,
		LoadCurve: []safety.LoadCurvePoint{
			{RadiusMM: 20_000, MaxLoadG: 6_000_000},
			{RadiusMM: 60_000, MaxLoadG: 1_500_000},
		},
	}
	bound := base
	bound.Version = 3
	staged := base
	staged.Version = 4
	// A looser curve staged as v4 would not trip the interlock v3 should trigger.
	staged.LoadCurve = []safety.LoadCurvePoint{
		{RadiusMM: 20_000, MaxLoadG: 8_000_000},
		{RadiusMM: 60_000, MaxLoadG: 3_000_000},
	}
	store.SeedConfig(bound)
	store.SeedConfig(staged)

	got, err := store.FindConfig(ctx, bound.ID, bound.Version)
	if err != nil {
		t.Fatalf("FindConfig(%s, %d) error = %v", bound.ID, bound.Version, err)
	}
	if got.Version != bound.Version {
		t.Fatalf("got version %d, want exactly bound version %d (resolved staged v4 instead)", got.Version, bound.Version)
	}
	if got.LoadCurve[0].MaxLoadG != bound.LoadCurve[0].MaxLoadG {
		t.Fatalf("got load curve %v, want bound curve %v — staged curve leaked into bound resolution", got.LoadCurve, bound.LoadCurve)
	}
}
