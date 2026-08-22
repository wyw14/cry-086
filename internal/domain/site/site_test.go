package site

import (
	"testing"
	"time"
)

func TestCraneStatusTransitions(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		from CraneStatus
		to   CraneStatus
		ok   bool
	}{
		{"commissioning to running", CraneCommissioning, CraneRunning, true},
		{"running to faulted", CraneRunning, CraneFaulted, true},
		{"faulted to stopped", CraneFaulted, CraneStopped, true},
		{"running cannot retire", CraneRunning, CraneRetired, false},
		{"retired is terminal", CraneRetired, CraneRunning, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			crane := TowerCrane{ID: "c", Status: test.from, Version: 4}
			err := crane.Transition(test.to, now)
			if (err == nil) != test.ok {
				t.Fatalf("Transition() error = %v, want success %v", err, test.ok)
			}
			if test.ok && crane.Version != 5 {
				t.Fatalf("version = %d, want 5", crane.Version)
			}
		})
	}
}
