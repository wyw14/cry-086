package httptransport

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-086/internal/middleware"
	"github.com/wyw14/cry-086/internal/platform/token"
	"go.uber.org/zap"
)

type RouterConfig struct {
	AllowedOrigins []string
	SimulatorKey   string
	WebDist        string
}

type safetyHTTP struct {
	engine     *gin.Engine
	settings   RouterConfig
	metrics    *middleware.Metrics
	guard      *middleware.Guard
	auth       *AuthHandler
	telemetry  *TelemetryHandler
	operations *OperationsHandler
	evidence   *EvidenceHandler
	ready      func(context.Context) error
}

func NewRouter(settings RouterConfig, logger *zap.Logger, metrics *middleware.Metrics, signer *token.Signer, auth *AuthHandler, telemetry *TelemetryHandler, operations *OperationsHandler, evidence *EvidenceHandler, ready func(context.Context) error) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	guard := middleware.NewGuard(logger, metrics, signer, middleware.GuardPolicy{AllowedOrigins: settings.AllowedOrigins, RequestsPerIP: 300, Window: time.Minute})
	station := &safetyHTTP{
		engine: gin.New(), settings: settings, metrics: metrics, guard: guard,
		auth: auth, telemetry: telemetry, operations: operations, evidence: evidence, ready: ready,
	}
	station.installRuntimeGuards()
	station.mountServiceSignals()
	station.mountCraneSafetyAPI()
	station.mountConsole()
	return station.engine
}

func (s *safetyHTTP) installRuntimeGuards() {
	for _, guard := range s.guard.Runtime() {
		s.engine.Use(guard)
	}
}

func (s *safetyHTTP) mountServiceSignals() {
	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "component": "crane-safety"})
	})
	s.engine.GET("/readyz", func(c *gin.Context) {
		if err := s.ready(c.Request.Context()); err != nil {
			respondError(c, http.StatusServiceUnavailable, "NOT_READY", "storage is not ready", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "telemetry_accepting": true})
	})
	s.engine.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"crane_guard_requests_total":  s.metrics.Requests.Load(),
			"crane_guard_errors_total":    s.metrics.Errors.Load(),
			"crane_guard_requests_active": s.metrics.Active.Load(),
		})
	})
}

func (s *safetyHTTP) mountCraneSafetyAPI() {
	versionOne := s.engine.Group("/api/v1")
	identityEndpoints := versionOne.Group("/auth")
	identityEndpoints.POST("/login", s.auth.Login)
	identityEndpoints.POST("/refresh", s.auth.Refresh)
	identityEndpoints.POST("/revoke", s.auth.Revoke)

	deviceIngress := versionOne.Group("/telemetry")
	deviceIngress.Use(middleware.DeviceKey(s.settings.SimulatorKey))
	deviceIngress.POST("", s.telemetry.Ingest)

	operatorEndpoints := versionOne.Group("")
	operatorEndpoints.Use(s.guard.RequireOperator())
	s.mountLiveOperations(operatorEndpoints)
	s.mountAssuranceOperations(operatorEndpoints)
}

func (s *safetyHTTP) mountLiveOperations(routes *gin.RouterGroup) {
	routes.GET("/cranes/:crane_id/snapshot", s.telemetry.Snapshot)
	routes.GET("/sites/:site_id/dashboard", s.operations.Dashboard)
	routes.POST("/alarms/:alarm_id/acknowledge", s.operations.Acknowledge)
	routes.POST("/alarms/:alarm_id/recovery", s.operations.BeginRecovery)
	routes.POST("/alarms/:alarm_id/resolve", s.operations.Resolve)
	routes.POST("/alarms/:alarm_id/manual-takeover", s.operations.ManualTakeover)
	routes.POST("/sites/:site_id/fleet/simulate", s.operations.SimulateFleet)
}

func (s *safetyHTTP) mountAssuranceOperations(routes *gin.RouterGroup) {
	routes.POST("/work-orders/:work_order_id/complete", s.operations.CompleteMaintenance)
	routes.POST("/cranes/:crane_id/resume", s.operations.ResumeCrane)
	routes.POST("/sites/:site_id/reports", s.operations.GenerateReport)
	routes.GET("/sites/:site_id/reports", s.operations.ListReports)
	routes.GET("/sites/:site_id/replay", s.operations.Replay)
	routes.POST("/sites/:site_id/files", s.evidence.Upload)
	routes.GET("/sites/:site_id/files/:file_id", s.evidence.Download)
}

func (s *safetyHTTP) mountConsole() {
	webRoot, err := filepath.Abs(s.settings.WebDist)
	if err != nil {
		return
	}
	info, err := os.Stat(webRoot)
	if err != nil || !info.IsDir() {
		return
	}
	s.engine.Static("/assets", filepath.Join(webRoot, "assets"))
	s.engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			respondError(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "API route does not exist", nil)
			return
		}
		c.File(filepath.Join(webRoot, "index.html"))
	})
}
