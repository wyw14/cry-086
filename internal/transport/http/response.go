package httptransport

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	FieldErrors []FieldError `json:"field_errors"`
	RequestID   string       `json:"request_id"`
}

func respondError(ctx *gin.Context, status int, code, message string, err error) {
	fields := make([]FieldError, 0)
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		for _, validationError := range validationErrors {
			fields = append(fields, FieldError{Field: validationError.Field(), Rule: validationError.Tag(), Message: "field did not satisfy " + validationError.Tag()})
		}
	}
	ctx.AbortWithStatusJSON(status, ErrorResponse{Code: code, Message: message, FieldErrors: fields, RequestID: ctx.GetString("request_id")})
}

func respondOK(ctx *gin.Context, status int, data any) {
	ctx.JSON(status, gin.H{"data": data, "request_id": ctx.GetString("request_id")})
}

func bindJSON(ctx *gin.Context, validate *validator.Validate, target any) bool {
	if err := ctx.ShouldBindJSON(target); err != nil {
		respondError(ctx, http.StatusBadRequest, "INVALID_JSON", "request body is invalid", err)
		return false
	}
	if err := validate.Struct(target); err != nil {
		respondError(ctx, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "request fields are invalid", err)
		return false
	}
	return true
}

func actorID(ctx *gin.Context) string { return ctx.GetString("actor_id") }
