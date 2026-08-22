package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/config"
)

func testApp(t *testing.T) *App {
	t.Helper()
	app, err := New(context.Background(), config.Config{
		HTTP:        config.HTTP{Address: ":0", RequestTimeout: 5 * time.Second, ShutdownTimeout: time.Second, AllowedOrigins: []string{"http://localhost"}},
		Storage:     config.Storage{Mode: "memory", EvidenceRoot: t.TempDir(), MaxEvidenceBytes: 1024 * 1024},
		Credentials: config.Credentials{AccessTokenSecret: "test-secret-with-more-than-24-characters", SimulatorKey: "sim-key"},
		SeedDemo:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(app.Close)
	return app
}

func loginToken(t *testing.T, app *App) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"safety","password":"CraneGuard!2026"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data.AccessToken
}

func TestAuthenticatedDashboardUsesResourceOwnership(t *testing.T) {
	app := testApp(t)
	token := loginToken(t, app)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sites/site-demo/dashboard?limit=20&sort=risk", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"site_id":"site-demo"`) || !strings.Contains(response.Body.String(), `"level":"degraded"`) {
		t.Fatalf("dashboard did not expose explicit degraded state: %s", response.Body.String())
	}
}

func TestTelemetryHTTPRejectsUnknownQuality(t *testing.T) {
	app := testApp(t)
	payload := map[string]any{"event_id": "bad-1", "crane_id": "crane-a", "sensor_id": "missing-sensor", "kind": "load", "value": 100, "unit": "kg", "sequence": 1, "observed_at": time.Now().UTC(), "simulator_node": "test"}
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Simulator-Key", "sim-key")
	response := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("telemetry status = %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "TELEMETRY_REJECTED") {
		t.Fatalf("stable error missing: %s", response.Body.String())
	}
}

func TestErrorResponseIncludesRequestID(t *testing.T) {
	app := testApp(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sites/site-demo/dashboard", nil)
	request.Header.Set("X-Request-ID", "client-request-1")
	response := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"request_id":"client-request-1"`) {
		t.Fatalf("request id missing: %s", response.Body.String())
	}
}
