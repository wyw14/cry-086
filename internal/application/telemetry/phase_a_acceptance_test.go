package telemetryapp

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
	platformclock "github.com/wyw14/cry-086/internal/platform/clock"
	"github.com/wyw14/cry-086/internal/repository/memory"
)

func TestIngestRejectsReadingOutsideReorderWindow(t *testing.T) {
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	clock := platformclock.NewFixed(now)
	repository := memory.New()
	crane, err := site.NewCrane("crane-1", "site-1", "SN-1", "model-1", "config-1", "unit-1", 1, site.Position{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateCrane(context.Background(), crane); err != nil {
		t.Fatal(err)
	}
	repository.SeedSensor(sensor.Sensor{ID: "load-1", CraneID: crane.ID, Kind: sensor.Load, NativeUnit: "g", CanonicalUnit: "g", MinValue: 0, MaxValue: 10_000_000, CalibrationID: "cal-1", CalibrationVer: 1, Communication: sensor.CommunicationOnline})
	repository.SeedCalibration(sensor.Calibration{ID: "cal-1", SensorID: "load-1", Version: 1, ScaleMicros: 1_000_000, EffectiveAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)})
	service := New(repository, StandardConverter{}, clock)
	first := telemetry.RawReading{EventID: "event-current", CraneID: crane.ID, SensorID: "load-1", Kind: sensor.Load, Value: 2_000_000, Unit: "g", Sequence: 20, ObservedAt: now, SimulatorNode: "node-a"}
	if _, err := service.Ingest(context.Background(), first); err != nil {
		t.Fatalf("seed current reading: %v", err)
	}
	old := telemetry.RawReading{EventID: "event-old", CraneID: crane.ID, SensorID: "load-1", Kind: sensor.Load, Value: 8_000_000, Unit: "g", Sequence: 10, ObservedAt: now.Add(time.Second), SimulatorNode: "node-a"}
	if _, err := service.Ingest(context.Background(), old); err == nil {
		t.Fatal("reading outside reorder window was accepted")
	}
	if repository.QuarantinedCount() != 1 {
		t.Fatalf("quarantined=%d want 1", repository.QuarantinedCount())
	}
	readings, err := repository.RecentReadings(context.Background(), crane.ID, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 1 {
		t.Fatalf("stored readings=%d want 1", len(readings))
	}
}
