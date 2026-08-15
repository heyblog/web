package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/config"
)

const RequestIDHeader = "X-Request-ID"

const requestIDContextKey = "heyblog.request_id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$`)

func requestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}
		ctx.Set(requestIDContextKey, requestID)
		ctx.Header(RequestIDHeader, requestID)
		ctx.Next()
	}
}

func RequestID(ctx *gin.Context) string {
	value, exists := ctx.Get(requestIDContextKey)
	if !exists {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

func newRequestID() string {
	buffer := make([]byte, 16)
	// crypto/rand.Read always fills the buffer and never returns an error.
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

func logAccess(ctx *gin.Context, logger *slog.Logger, started time.Time) {
	status := ctx.Writer.Status()
	level := slog.LevelInfo
	switch {
	case status >= http.StatusInternalServerError:
		level = slog.LevelError
	case status >= http.StatusBadRequest:
		level = slog.LevelWarn
	case strings.HasPrefix(ctx.Request.URL.Path, "/health/"):
		level = slog.LevelDebug
	}
	route := ctx.FullPath()
	if route == "" {
		route = ctx.Request.URL.Path
	}
	logger.LogAttrs(ctx.Request.Context(), level, "HTTP request",
		slog.String("event", "http_request"),
		slog.String("request_id", RequestID(ctx)),
		slog.String("method", ctx.Request.Method),
		slog.String("route", route),
		slog.Int("status", status),
		slog.Int("response_bytes", ctx.Writer.Size()),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		slog.String("client_ip", ctx.ClientIP()),
	)
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Referrer-Policy", "no-referrer")
		ctx.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		ctx.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		ctx.Next()
	}
}

func corsMiddleware(configuration config.CORSConfig) gin.HandlerFunc {
	allowedMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	allowedHeaders := []string{"Content-Type", "Authorization", RequestIDHeader}
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin == "" {
			ctx.Next()
			return
		}
		addVary(ctx, "Origin")
		if !slices.Contains(configuration.AllowOrigins, origin) {
			_ = ctx.Error(apperror.New(apperror.KindForbidden, apperror.CodeForbidden, "cross-origin request is not allowed"))
			ctx.Abort()
			return
		}

		ctx.Header("Access-Control-Allow-Origin", origin)
		if configuration.AllowCredentials {
			ctx.Header("Access-Control-Allow-Credentials", "true")
		}
		if ctx.Request.Method != http.MethodOptions {
			ctx.Next()
			return
		}

		addVary(ctx, "Access-Control-Request-Method")
		addVary(ctx, "Access-Control-Request-Headers")
		requestedMethod := ctx.GetHeader("Access-Control-Request-Method")
		if !slices.Contains(allowedMethods, requestedMethod) || !headersAllowed(ctx.GetHeader("Access-Control-Request-Headers"), allowedHeaders) {
			_ = ctx.Error(apperror.New(apperror.KindForbidden, apperror.CodeForbidden, "cross-origin preflight is not allowed"))
			ctx.Abort()
			return
		}
		ctx.Header("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
		ctx.Header("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
		ctx.AbortWithStatus(http.StatusNoContent)
	}
}

func bodyLimitMiddleware(defaultLimit int64, overrides ...map[Route]int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		limit := defaultLimit
		overridden := false
		if len(overrides) > 0 {
			if override, exists := overrides[0][Route{Method: ctx.Request.Method, Path: ctx.FullPath()}]; exists {
				limit = override
				overridden = true
			}
		}
		// Route-specific limits are commonly used by authenticated upload
		// endpoints. Defer their declared-length failure until the handler reads
		// the wrapped body so route authorization always runs first.
		if !overridden && ctx.Request.ContentLength > limit {
			_ = ctx.Error(&http.MaxBytesError{Limit: limit})
			ctx.Abort()
			return
		}
		if ctx.Request.Body != nil {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
		}
		ctx.Next()
	}
}

func headersAllowed(requested string, allowed []string) bool {
	for _, header := range strings.Split(requested, ",") {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		matched := false
		for _, candidate := range allowed {
			if strings.EqualFold(header, candidate) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func addVary(ctx *gin.Context, value string) {
	for _, current := range ctx.Writer.Header().Values("Vary") {
		if strings.EqualFold(current, value) {
			return
		}
	}
	ctx.Writer.Header().Add("Vary", value)
}
