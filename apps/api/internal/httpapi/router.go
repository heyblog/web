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
	Mode             config.Mode
	HTTP             config.HTTPConfig
	Logger           *slog.Logger
	Health           *Health
	HealthcheckToken string
}

func NewRouter(options Options) (*gin.Engine, error) {
	if options.HealthcheckToken == "" {
		return nil, fmt.Errorf("healthcheck token is required")
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
		bodyLimitMiddleware(options.HTTP.MaxBodyBytes),
	)

	health := options.Health
	if health == nil {
		health = NewHealth(nil, 0)
	}
	router.GET("/ping", Adapt(func(*Context) (Response, error) {
		return JSON(http.StatusOK, map[string]string{"message": "pong"})
	}))
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
