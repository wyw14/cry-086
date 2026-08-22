package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-086/internal/platform/token"
	"go.uber.org/zap"
)

type Metrics struct {
	Requests atomic.Uint64
	Errors   atomic.Uint64
	Active   atomic.Int64
}

type GuardPolicy struct {
	AllowedOrigins []string
	RequestsPerIP  int
	Window         time.Duration
}

type Guard struct {
	logger   *zap.Logger
	metrics  *Metrics
	signer   *token.Signer
	origins  map[string]struct{}
	limit    int
	window   time.Duration
	now      func() time.Time
	sequence atomic.Uint64
	traffic  atomic.Uint64
	mu       sync.Mutex
	clients  map[string]clientWindow
}

type clientWindow struct {
	used  int
	until time.Time
}

func NewGuard(logger *zap.Logger, metrics *Metrics, signer *token.Signer, policy GuardPolicy) *Guard {
	origins := make(map[string]struct{}, len(policy.AllowedOrigins))
	for _, origin := range policy.AllowedOrigins {
		origins[origin] = struct{}{}
	}
	return &Guard{
		logger: logger, metrics: metrics, signer: signer, origins: origins,
		limit: policy.RequestsPerIP, window: policy.Window, now: time.Now,
		clients: make(map[string]clientWindow),
	}
}

func (g *Guard) Runtime() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		g.identifyRequest,
		g.addBrowserProtection,
		g.applyOriginPolicy,
		g.enforceClientBudget,
		g.recoverPanics(),
		g.observeRequest,
	}
}

func (g *Guard) RequireOperator() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		const prefix = "Bearer "
		header := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(header, prefix) {
			g.reject(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "access token is required")
			return
		}
		claims, err := g.signer.Verify(strings.TrimSpace(strings.TrimPrefix(header, prefix)), g.now().UTC())
		if err != nil {
			g.reject(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "access token is invalid")
			return
		}
		ctx.Set("actor_id", claims.Subject)
		ctx.Set("roles", claims.Roles)
		ctx.Set("site_ids", claims.SiteIDs)
		ctx.Next()
	}
}

func (g *Guard) identifyRequest(ctx *gin.Context) {
	requestID := strings.TrimSpace(ctx.GetHeader("X-Request-ID"))
	if requestID == "" || len(requestID) > 100 || strings.ContainsAny(requestID, "\r\n") {
		requestID = fmt.Sprintf("crane-%x-%d", g.now().UTC().UnixNano(), g.sequence.Add(1))
	}
	ctx.Set("request_id", requestID)
	ctx.Header("X-Request-ID", requestID)
	ctx.Next()
}

func (g *Guard) addBrowserProtection(ctx *gin.Context) {
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("X-Frame-Options", "DENY")
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.Header("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'")
	ctx.Next()
}

func (g *Guard) applyOriginPolicy(ctx *gin.Context) {
	origin := ctx.GetHeader("Origin")
	_, permitted := g.origins[origin]
	if origin != "" && permitted {
		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Vary", "Origin")
		ctx.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,X-Request-ID")
		ctx.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
	}
	if ctx.Request.Method == http.MethodOptions {
		if origin != "" && !permitted {
			g.reject(ctx, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		ctx.AbortWithStatus(http.StatusNoContent)
		return
	}
	ctx.Next()
}

func (g *Guard) enforceClientBudget(ctx *gin.Context) {
	if g.limit <= 0 || g.window <= 0 {
		ctx.Next()
		return
	}
	now := g.now().UTC()
	key := ctx.ClientIP()
	g.mu.Lock()
	current := g.clients[key]
	if !now.Before(current.until) {
		current = clientWindow{until: now.Add(g.window)}
	}
	current.used++
	g.clients[key] = current
	if g.traffic.Add(1)%1024 == 0 {
		for client, budget := range g.clients {
			if !now.Before(budget.until) {
				delete(g.clients, client)
			}
		}
	}
	g.mu.Unlock()
	if current.used > g.limit {
		g.reject(ctx, http.StatusTooManyRequests, "RATE_LIMITED", "request rate exceeded")
		return
	}
	ctx.Next()
}

func (g *Guard) recoverPanics() gin.HandlerFunc {
	return gin.CustomRecovery(func(ctx *gin.Context, recovered any) {
		g.metrics.Errors.Add(1)
		g.logger.Error("http_panic", zap.String("request_id", ctx.GetString("request_id")), zap.Any("panic", recovered))
		g.reject(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred")
	})
}

func (g *Guard) observeRequest(ctx *gin.Context) {
	started := g.now()
	g.metrics.Requests.Add(1)
	g.metrics.Active.Add(1)
	defer g.metrics.Active.Add(-1)
	ctx.Next()
	if ctx.Writer.Status() >= http.StatusInternalServerError {
		g.metrics.Errors.Add(1)
	}
	g.logger.Info("crane_api_request",
		zap.String("request_id", ctx.GetString("request_id")),
		zap.String("method", ctx.Request.Method),
		zap.String("route", ctx.FullPath()),
		zap.Int("status", ctx.Writer.Status()),
		zap.Duration("elapsed", g.now().Sub(started)),
	)
}

func (g *Guard) reject(ctx *gin.Context, status int, code, message string) {
	ctx.AbortWithStatusJSON(status, gin.H{
		"code": code, "message": message, "field_errors": []any{}, "request_id": ctx.GetString("request_id"),
	})
}
