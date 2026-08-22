package httptransport

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	alarmapp "github.com/wyw14/cry-086/internal/application/alarm"
	"github.com/wyw14/cry-086/internal/application/dashboard"
	fleetapp "github.com/wyw14/cry-086/internal/application/fleet"
	maintenanceapp "github.com/wyw14/cry-086/internal/application/maintenance"
	reportapp "github.com/wyw14/cry-086/internal/application/report"
	"github.com/wyw14/cry-086/internal/domain/report"
	"github.com/wyw14/cry-086/internal/domain/safety"
)

type OperationsHandler struct {
	alarms      *alarmapp.Service
	dashboard   *dashboard.Service
	fleet       *fleetapp.Service
	maintenance *maintenanceapp.Service
	reports     *reportapp.Service
	validate    *validator.Validate
}

func NewOperationsHandler(alarms *alarmapp.Service, dashboardService *dashboard.Service, fleetService *fleetapp.Service, maintenanceService *maintenanceapp.Service, reports *reportapp.Service, validate *validator.Validate) *OperationsHandler {
	return &OperationsHandler{alarms: alarms, dashboard: dashboardService, fleet: fleetService, maintenance: maintenanceService, reports: reports, validate: validate}
}

func (h *OperationsHandler) Dashboard(ctx *gin.Context) {
	limit := queryInt(ctx, "limit", 20)
	filters := make(map[string]string)
	for _, key := range []string{"status", "model_id", "responsible_unit_id"} {
		if value := ctx.Query(key); value != "" {
			filters[key] = value
		}
	}
	view, err := h.dashboard.Load(ctx.Request.Context(), actorID(ctx), ctx.Param("site_id"), ctx.Query("cursor"), limit, defaultValue(ctx.Query("sort"), "serial_number"), filters)
	if err != nil {
		respondError(ctx, http.StatusForbidden, "DASHBOARD_UNAVAILABLE", "dashboard could not be loaded", err)
		return
	}
	respondOK(ctx, http.StatusOK, view)
}

func (h *OperationsHandler) Acknowledge(ctx *gin.Context) {
	var request struct {
		Version int64 `json:"version" validate:"required,gte=1"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	event, err := h.alarms.Acknowledge(ctx.Request.Context(), ctx.Param("alarm_id"), actorID(ctx), ctx.GetString("request_id"), request.Version)
	if err != nil {
		respondError(ctx, http.StatusConflict, "ALARM_ACK_FAILED", "alarm could not be acknowledged", err)
		return
	}
	respondOK(ctx, http.StatusOK, event)
}

func (h *OperationsHandler) BeginRecovery(ctx *gin.Context) {
	var request struct {
		Version int64 `json:"version" validate:"required,gte=1"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	event, err := h.alarms.BeginRecovery(ctx.Request.Context(), ctx.Param("alarm_id"), actorID(ctx), ctx.GetString("request_id"), request.Version)
	if err != nil {
		respondError(ctx, http.StatusConflict, "RECOVERY_START_FAILED", "alarm recovery could not start", err)
		return
	}
	respondOK(ctx, http.StatusOK, event)
}

func (h *OperationsHandler) Resolve(ctx *gin.Context) {
	var request struct {
		Version      int64  `json:"version" validate:"required,gte=1"`
		Reason       string `json:"reason" validate:"required,min=5,max=500"`
		ConfigID     string `json:"config_id" validate:"required"`
		ConfigVer    int64  `json:"config_version" validate:"required,gte=1"`
		RecoveryHold int64  `json:"recovery_hold_seconds" validate:"required,gte=1,lte=3600"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	config := safety.Config{ID: request.ConfigID, Version: request.ConfigVer, RecoveryHold: time.Duration(request.RecoveryHold) * time.Second}
	event, err := h.alarms.Resolve(ctx.Request.Context(), ctx.Param("alarm_id"), actorID(ctx), ctx.GetString("request_id"), request.Reason, request.Version, config)
	if err != nil {
		respondError(ctx, http.StatusConflict, "ALARM_RESOLVE_FAILED", "alarm recovery conditions were not satisfied", err)
		return
	}
	respondOK(ctx, http.StatusOK, event)
}

func (h *OperationsHandler) ManualTakeover(ctx *gin.Context) {
	var request struct {
		Version int64  `json:"version" validate:"required,gte=1"`
		Reason  string `json:"reason" validate:"required,min=5,max=500"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	event, err := h.alarms.ManualTakeover(ctx.Request.Context(), ctx.Param("alarm_id"), actorID(ctx), ctx.GetString("request_id"), request.Reason, request.Version)
	if err != nil {
		respondError(ctx, http.StatusForbidden, "MANUAL_TAKEOVER_FAILED", "manual takeover was not authorized", err)
		return
	}
	respondOK(ctx, http.StatusOK, event)
}

func (h *OperationsHandler) SimulateFleet(ctx *gin.Context) {
	var request struct {
		Velocities map[string][2]int64 `json:"velocities" validate:"required,min=2"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	risks, err := h.fleet.Simulate(ctx.Request.Context(), ctx.Param("site_id"), request.Velocities)
	if err != nil {
		respondError(ctx, http.StatusConflict, "FLEET_SIMULATION_FAILED", "local fleet trajectory simulation failed", err)
		return
	}
	respondOK(ctx, http.StatusOK, risks)
}

func (h *OperationsHandler) CompleteMaintenance(ctx *gin.Context) {
	var request struct {
		Version    int64  `json:"version" validate:"required,gte=1"`
		Result     string `json:"result" validate:"required,min=3,max=1000"`
		EvidenceID string `json:"evidence_file_id" validate:"max=100"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	order, err := h.maintenance.CompleteAndStop(ctx.Request.Context(), ctx.Param("work_order_id"), actorID(ctx), ctx.GetString("request_id"), request.Result, request.EvidenceID, request.Version)
	if err != nil {
		respondError(ctx, http.StatusConflict, "MAINTENANCE_COMPLETE_FAILED", "work order could not be completed", err)
		return
	}
	respondOK(ctx, http.StatusOK, order)
}

func (h *OperationsHandler) ResumeCrane(ctx *gin.Context) {
	var request struct {
		Version            int64 `json:"version" validate:"required,gte=1"`
		SafetyReviewPassed bool  `json:"safety_review_passed"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	crane, err := h.maintenance.ResumeCrane(ctx.Request.Context(), ctx.Param("crane_id"), actorID(ctx), ctx.GetString("request_id"), request.Version, request.SafetyReviewPassed)
	if err != nil {
		respondError(ctx, http.StatusConflict, "CRANE_RESUME_FAILED", "crane could not resume", err)
		return
	}
	respondOK(ctx, http.StatusOK, crane)
}

func (h *OperationsHandler) GenerateReport(ctx *gin.Context) {
	var request struct {
		Start         time.Time `json:"start" validate:"required"`
		End           time.Time `json:"end" validate:"required"`
		RetentionDays int       `json:"retention_days" validate:"required,gte=30,lte=3650"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	generated, err := h.reports.GenerateRegulatory(ctx.Request.Context(), actorID(ctx), report.ShiftWindow{SiteID: ctx.Param("site_id"), Start: request.Start, End: request.End}, request.RetentionDays)
	if err != nil {
		respondError(ctx, http.StatusConflict, "REPORT_GENERATION_FAILED", "regulatory report could not be generated", err)
		return
	}
	respondOK(ctx, http.StatusCreated, generated)
}

func (h *OperationsHandler) Replay(ctx *gin.Context) {
	start, startErr := time.Parse(time.RFC3339, ctx.Query("start"))
	end, endErr := time.Parse(time.RFC3339, ctx.Query("end"))
	if startErr != nil || endErr != nil {
		respondError(ctx, http.StatusBadRequest, "INVALID_TIME_RANGE", "start and end must be RFC3339 timestamps", errors.Join(startErr, endErr))
		return
	}
	events, err := h.reports.Replay(ctx.Request.Context(), actorID(ctx), ctx.Param("site_id"), start, end, queryInt(ctx, "limit", 100))
	if err != nil {
		respondError(ctx, http.StatusForbidden, "REPLAY_UNAVAILABLE", "event replay could not be loaded", err)
		return
	}
	respondOK(ctx, http.StatusOK, events)
}

func (h *OperationsHandler) ListReports(ctx *gin.Context) {
	values, next, err := h.reports.List(ctx.Request.Context(), actorID(ctx), ctx.Param("site_id"), ctx.Query("cursor"), queryInt(ctx, "limit", 50))
	if err != nil {
		respondError(ctx, http.StatusForbidden, "REPORTS_UNAVAILABLE", "reports could not be listed", err)
		return
	}
	respondOK(ctx, http.StatusOK, gin.H{"items": values, "next_cursor": next})
}

func queryInt(ctx *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
