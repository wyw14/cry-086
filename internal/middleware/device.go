package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

func DeviceKey(expected string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		provided := ctx.GetHeader("X-Simulator-Key")
		if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "DEVICE_UNAUTHORIZED", "message": "simulator key is invalid", "field_errors": []any{}, "request_id": ctx.GetString("request_id")})
			return
		}
		ctx.Next()
	}
}
