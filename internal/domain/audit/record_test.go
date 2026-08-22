package audit

import (
	"testing"
	"time"
)

func TestAuditRecordDetectsMutation(t *testing.T) {
	record := New("id", "site", "actor", "api", "alarm.resolved", "alarm", "a1", "safe", "request", map[string]any{"status": "recovering"}, map[string]any{"status": "resolved"}, time.Now(), "")
	if !record.Verify() {
		t.Fatal("new audit record did not verify")
	}
	record.After["status"] = "open"
	if record.Verify() {
		t.Fatal("mutated audit record still verified")
	}
}
