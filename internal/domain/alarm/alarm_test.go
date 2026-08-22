package alarm

import (
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/safety"
)

func TestHighRiskRecoveryRequiresAuthorizationAndHold(t *testing.T) {
	now := time.Now().UTC()
	event, err := New("alarm", "site", "crane", "telemetry", safety.Decision{Level: safety.RiskCritical, Interlock: true}, "evidence", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Acknowledge("operator", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := event.BeginRecovery(now.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := event.Resolve("operator", false, now.Add(2*time.Second), now.Add(time.Minute), 30*time.Second); err == nil {
		t.Fatal("unauthorized resolution succeeded")
	}
	if err := event.Resolve("officer", true, now.Add(50*time.Second), now.Add(time.Minute), 30*time.Second); err == nil {
		t.Fatal("recovery shorter than hold succeeded")
	}
	if err := event.Resolve("officer", true, now.Add(2*time.Second), now.Add(time.Minute), 30*time.Second); err != nil {
		t.Fatalf("authorized recovery failed: %v", err)
	}
	if event.Interlock {
		t.Fatal("resolved event retained interlock")
	}
}
