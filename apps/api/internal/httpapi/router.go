package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/config"
)

type Options struct {
	Mode               config.Mode
	HTTP               config.HTTPConfig
	Logger             *slog.Logger
	Health             *Health
	HealthcheckToken   string
	WebToken           string
	BodyLimitOverrides map[Route]int64
}

type Route struct {
	Method string
	Path   string
}

type endpointAudience uint8

const (
	endpointAudienceWeb endpointAudience = iota + 1
	endpointAudiencePublic
)

func NewRouter(options Options) (*gin.Engine, error) {
	if options.HealthcheckToken == "" {
		return nil, fmt.Errorf("healthcheck token is required")
	}
	if options.WebToken == "" {
		return nil, fmt.Errorf("web token is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if options.Mode == config.ModeProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(options.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	router.Use(
		errorBoundary(logger),
		requestIDMiddleware(),
		securityHeadersMiddleware(),
		corsMiddleware(options.HTTP.CORS),
		bodyLimitMiddleware(options.HTTP.MaxBodyBytes, options.BodyLimitOverrides),
	)

	health := options.Health
	if health == nil {
		health = NewHealth(nil, 0)
	}
	ping, err := adaptApplicationEndpoint(endpointAudienceWeb, options.WebToken, func(*Context) (Response, error) {
		return JSON(http.StatusOK, map[string]string{"message": "pong"})
	})
	if err != nil {
		return nil, err
	}
	router.GET("/ping", ping)
	healthAuth := healthAuthorization(options.HealthcheckToken)
	router.GET("/health/live", Adapt(Chain(func(*Context) (Response, error) {
		return NoContent(http.StatusNoContent).WithHeader("Cache-Control", "no-store"), nil
	}, healthAuth)))
	router.GET("/health/ready", Adapt(Chain(func(ctx *Context) (Response, error) {
		if err := health.Ready(ctx.Request.Context()); err != nil {
			return Response{}, apperror.Wrap(
				err,
				apperror.KindUnavailable,
				apperror.CodeServiceUnavailable,
				"service is not ready",
				"check service readiness",
			)
		}
		return NoContent(http.StatusNoContent).WithHeader("Cache-Control", "no-store"), nil
	}, healthAuth)))

	router.NoRoute(Adapt(func(*Context) (Response, error) {
		return Response{}, apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "the requested resource was not found")
	}))
	router.NoMethod(Adapt(func(*Context) (Response, error) {
		return Response{}, apperror.New(apperror.KindMethodNotAllowed, apperror.CodeMethodNotAllowed, "the method is not allowed for this resource")
	}))
	return router, nil
}

func adaptApplicationEndpoint(
	audience endpointAudience,
	webToken string,
	endpoint Endpoint,
) (gin.HandlerFunc, error) {
	switch audience {
	case endpointAudienceWeb:
		return Adapt(Chain(endpoint, webAuthorization(webToken))), nil
	case endpointAudiencePublic:
		return Adapt(endpoint), nil
	default:
		return nil, fmt.Errorf("unsupported endpoint audience: %d", audience)
	}
}
