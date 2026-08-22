package telemetry

import (
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
)

func TestSnapshotDegradesWhenMandatoryMeasurementIsMissing(t *testing.T) {
	now := time.Now().UTC()
	readings := []NormalizedReading{
		{EventID: "load", Kind: sensor.Load, ObservedAt: now, Quality: QualityGood},
		{EventID: "radius", Kind: sensor.Radius, ObservedAt: now, Quality: QualityGood},
	}
	snapshot := NewSnapshot("crane", now, readings, 10*time.Second)
	if snapshot.OverallQuality != QualityDegraded {
		t.Fatalf("quality = %s, want degraded", snapshot.OverallQuality)
	}
}

func TestSnapshotKeepsLatestReadingByKind(t *testing.T) {
	now := time.Now().UTC()
	readings := []NormalizedReading{
		{EventID: "new", Kind: sensor.Load, Value: 20, ObservedAt: now, Quality: QualityGood},
		{EventID: "old", Kind: sensor.Load, Value: 10, ObservedAt: now.Add(-time.Second), Quality: QualityGood},
	}
	snapshot := NewSnapshot("crane", now, readings, time.Minute)
	if snapshot.Readings[sensor.Load].EventID != "new" {
		t.Fatal("out-of-order input replaced the latest reading")
	}
}
