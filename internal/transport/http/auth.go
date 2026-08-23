package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	authapp "github.com/wyw14/cry-086/internal/application/auth"
)

type AuthHandler struct {
	service  *authapp.Service
	validate *validator.Validate
}

func NewAuthHandler(service *authapp.Service, validate *validator.Validate) *AuthHandler {
	return &AuthHandler{service: service, validate: validate}
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var request struct {
		Username string `json:"username" validate:"required,min=3,max=100"`
		Password string `json:"password" validate:"required,min=12,max=128"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	tokens, err := h.service.Login(ctx.Request.Context(), request.Username, request.Password)
	if err != nil {
		respondError(ctx, http.StatusUnauthorized, "INVALID_CREDENTIALS", "username or password is invalid", err)
		return
	}
	respondOK(ctx, http.StatusOK, tokens)
}

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	var request struct {
		RefreshToken string `json:"refresh_token" validate:"required,max=500"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	tokens, err := h.service.Refresh(ctx.Request.Context(), request.RefreshToken)
	if err != nil {
		respondError(ctx, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "refresh token is invalid", err)
		return
	}
	respondOK(ctx, http.StatusOK, tokens)
}

func (h *AuthHandler) Revoke(ctx *gin.Context) {
	var request struct {
		RefreshToken string `json:"refresh_token" validate:"required,max=500"`
	}
	if !bindJSON(ctx, h.validate, &request) {
		return
	}
	if err := h.service.Revoke(ctx.Request.Context(), request.RefreshToken); err != nil {
		respondError(ctx, http.StatusConflict, "REFRESH_REVOKE_FAILED", "refresh token could not be revoked", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
