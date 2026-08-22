package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	safetyapp "github.com/wyw14/cry-086/internal/application/safety"
	telemetryapp "github.com/wyw14/cry-086/internal/application/telemetry"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
)

type TelemetryHandler struct {
	ingestion *telemetryapp.Service
	safety    *safetyapp.Service
	validate  *validator.Validate
}

func NewTelemetryHandler(ingestion *telemetryapp.Service, safetyService *safetyapp.Service, validate *validator.Validate) *TelemetryHandler {
	return &TelemetryHandler{ingestion: ingestion, safety: safetyService, validate: validate}
}

func (h *TelemetryHandler) Ingest(ctx *gin.Context) {
	var request telemetry.RawReading
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	normalized, err := h.ingestion.Ingest(ctx.Request.Context(), request)
	if err != nil {
		respondError(ctx, http.StatusUnprocessableEntity, "TELEMETRY_REJECTED", "telemetry was isolated because its quality could not be confirmed", err)
		return
	}
	if normalized.Quality == telemetry.QualityDuplicate {
		respondOK(ctx, http.StatusOK, gin.H{"reading": normalized, "duplicate": true, "interlock_reused": true})
		return
	}
	decision, event, err := h.safety.Evaluate(ctx.Request.Context(), normalized.CraneID, normalized.EventID, normalized.EvidenceChecksum)
	if err != nil {
		respondError(ctx, http.StatusConflict, "SAFETY_EVALUATION_FAILED", "telemetry was stored but safety evaluation failed", err)
		return
	}
	respondOK(ctx, http.StatusAccepted, gin.H{"reading": normalized, "decision": decision, "alarm": event})
}

func (h *TelemetryHandler) Snapshot(ctx *gin.Context) {
	snapshot, err := h.ingestion.Snapshot(ctx.Request.Context(), ctx.Param("crane_id"))
	if err != nil {
		respondError(ctx, http.StatusNotFound, "SNAPSHOT_NOT_FOUND", "current telemetry snapshot is unavailable", err)
		return
	}
	respondOK(ctx, http.StatusOK, snapshot)
}
