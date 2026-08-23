package sensor

import (
	"testing"
	"time"
)

func TestCalibrationEffectiveWindow(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	calibration := Calibration{ID: "cal", Version: 2, ScaleMicros: 1_100_000, OffsetMicros: -10, EffectiveAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	value, err := calibration.Apply(1000, now)
	if err != nil || value != 1090 {
		t.Fatalf("Apply() = %d, %v; want 1090, nil", value, err)
	}
	if _, err := calibration.Apply(1000, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired calibration was accepted")
	}
}

func TestCommunicationNeverPretendsUnknownIsOnline(t *testing.T) {
	now := time.Now().UTC()
	device := Sensor{}
	if state := device.Observe(now, 10*time.Second); state != CommunicationUnknown {
		t.Fatalf("state = %s, want unknown", state)
	}
	device.LastSeenAt = now.Add(-11 * time.Second)
	if state := device.Observe(now, 10*time.Second); state != CommunicationOffline {
		t.Fatalf("state = %s, want offline", state)
	}
}
