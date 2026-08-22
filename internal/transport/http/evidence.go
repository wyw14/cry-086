package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-086/internal/application/evidence"
)

type EvidenceHandler struct{ service *evidence.Service }

func NewEvidenceHandler(service *evidence.Service) *EvidenceHandler {
	return &EvidenceHandler{service: service}
}

func (h *EvidenceHandler) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		respondError(ctx, http.StatusBadRequest, "FILE_REQUIRED", "multipart file is required", err)
		return
	}
	source, err := file.Open()
	if err != nil {
		respondError(ctx, http.StatusBadRequest, "FILE_OPEN_FAILED", "uploaded file could not be opened", err)
		return
	}
	defer source.Close()
	metadata, err := h.service.Upload(ctx.Request.Context(), actorID(ctx), ctx.Param("site_id"), file.Filename, file.Header.Get("Content-Type"), source)
	if err != nil {
		respondError(ctx, http.StatusUnprocessableEntity, "FILE_REJECTED", "file failed validation or authorization", err)
		return
	}
	respondOK(ctx, http.StatusCreated, metadata)
}

func (h *EvidenceHandler) Download(ctx *gin.Context) {
	metadata, source, err := h.service.Download(ctx.Request.Context(), actorID(ctx), ctx.Param("site_id"), ctx.Param("file_id"))
	if err != nil {
		respondError(ctx, http.StatusForbidden, "FILE_UNAVAILABLE", "file could not be downloaded", err)
		return
	}
	defer source.Close()
	ctx.Header("Content-Disposition", `attachment; filename="`+metadata.Name+`"`)
	ctx.Header("X-Content-SHA256", metadata.Checksum)
	ctx.DataFromReader(http.StatusOK, metadata.Size, metadata.MIME, source, nil)
}
