package safety

import (
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
)

func TestEvaluateUsesDegradedStateWhenQualityUncertain(t *testing.T) {
	decision := Evaluate(Config{ID: "cfg", Version: 3}, telemetry.Snapshot{OverallQuality: telemetry.QualityDegraded}, time.Now())
	if decision.Level != RiskDegraded || decision.Interlock {
		t.Fatalf("decision = %#v, want explicit degraded without fabricated interlock", decision)
	}
}

func TestEvaluateCombinedLoadAndWind(t *testing.T) {
	now := time.Now().UTC()
	config := Config{ID: "cfg", Version: 3, LoadCurve: []LoadCurvePoint{{RadiusMM: 20_000, MaxLoadG: 6_000_000}, {RadiusMM: 60_000, MaxLoadG: 1_500_000}}, WarningWindMilliMPS: 12_000, StopWindMilliMPS: 15_000, CombinationMargin: 10}
	readings := map[sensor.Kind]telemetry.NormalizedReading{
		sensor.Load:        {EventID: "load", Kind: sensor.Load, Value: 1_500_000},
		sensor.Radius:      {EventID: "radius", Kind: sensor.Radius, Value: 55_000},
		sensor.WindSpeed:   {EventID: "wind", Kind: sensor.WindSpeed, Value: 16_000},
		sensor.LimitSwitch: {EventID: "limit", Kind: sensor.LimitSwitch},
	}
	decision := Evaluate(config, telemetry.Snapshot{OverallQuality: telemetry.QualityGood, Readings: readings}, now)
	if decision.Level != RiskCritical || !decision.Interlock {
		t.Fatalf("decision = %#v, want critical interlock", decision)
	}
	if decision.ConfigID != "cfg" || decision.ConfigVersion != 3 {
		t.Fatal("decision lost the exact safety configuration identity")
	}
}
