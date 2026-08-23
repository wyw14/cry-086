package safetyapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
	platformclock "github.com/wyw14/cry-086/internal/platform/clock"
	"github.com/wyw14/cry-086/internal/repository/memory"
)

type fixedSnapshot struct{ value telemetry.Snapshot }

func (f fixedSnapshot) Snapshot(context.Context, string) (telemetry.Snapshot, error) { return f.value, nil }

type sequenceIDs struct{ next int }

func (s *sequenceIDs) NewID() string { s.next++; return fmt.Sprintf("alarm-%d", s.next) }

func TestEvaluateUsesCraneBoundSafetyConfigVersion(t *testing.T) {
	now := time.Date(2026, 8, 23, 3, 0, 0, 0, time.UTC)
	repository := memory.New()
	crane, err := site.NewCrane("crane-versioned", "site-1", "SN-V", "model-v", "curve-v", "unit-1", 3, site.Position{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateCrane(context.Background(), crane); err != nil {
		t.Fatal(err)
	}
	repository.SeedConfig(safety.Config{ID: "curve-v", CraneModelID: "model-v", Version: 3, LoadCurve: []safety.LoadCurvePoint{{RadiusMM: 20_000, MaxLoadG: 1_000}, {RadiusMM: 40_000, MaxLoadG: 500}}, WarningWindMilliMPS: 10_000, StopWindMilliMPS: 15_000})
	repository.SeedConfig(safety.Config{ID: "curve-v", CraneModelID: "model-v", Version: 4, LoadCurve: []safety.LoadCurvePoint{{RadiusMM: 20_000, MaxLoadG: 5_000}, {RadiusMM: 40_000, MaxLoadG: 2_500}}, WarningWindMilliMPS: 10_000, StopWindMilliMPS: 15_000})
	readings := map[sensor.Kind]telemetry.NormalizedReading{}
	for _, kind := range []sensor.Kind{sensor.Load, sensor.Radius, sensor.Height, sensor.SlewAngle, sensor.WindSpeed, sensor.LimitSwitch} {
		readings[kind] = telemetry.NormalizedReading{EventID: "event-" + string(kind), Kind: kind, ObservedAt: now, Quality: telemetry.QualityGood}
	}
	load := readings[sensor.Load]
	load.Value = 1_500
	readings[sensor.Load] = load
	radius := readings[sensor.Radius]
	radius.Value = 20_000
	readings[sensor.Radius] = radius
	provider := fixedSnapshot{value: telemetry.Snapshot{CraneID: crane.ID, At: now, Readings: readings, OverallQuality: telemetry.QualityGood}}
	service := New(repository, provider, platformclock.NewFixed(now), &sequenceIDs{})
	decision, event, err := service.Evaluate(context.Background(), crane.ID, "trigger-v3", "evidence-v3")
	if err != nil {
		t.Fatal(err)
	}
	if decision.ConfigVersion != 3 || decision.Level != safety.RiskCritical || event == nil || !event.Interlock {
		t.Fatalf("decision=%+v event=%+v", decision, event)
	}
}
