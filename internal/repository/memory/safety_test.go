package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/safety"
)

// TestFindConfigHonorsBoundVersion guards against regressing to "latest version
// wins" semantics: a crane pinned to an older safety configuration version must
// resolve that exact version even when a newer one is staged, so safety rules
// are evaluated against the curve the device is actually bound to.
func TestFindConfigHonorsBoundVersion(t *testing.T) {
	store := New()
	ctx := context.Background()
	base := safety.Config{
		ID:                  "config-zt6015",
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

	if _, err := store.FindConfig(ctx, bound.ID, 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindConfig for unbound version 5 = %v, want ErrNotFound", err)
	}
}
