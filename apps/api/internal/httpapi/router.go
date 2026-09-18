package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/config"
)

type Options struct {
	Mode               config.Mode
	HTTP               config.HTTPConfig
	Logger             *slog.Logger
	Health             *Health
	HealthcheckToken   string
	WebToken           string
	PublicViews        publicview.Reader
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

func NewRouter(options Options) (*Router, error) {
	if options.HealthcheckToken == "" {
		return nil, fmt.Errorf("healthcheck token is required")
	}
	if options.WebToken == "" {
		return nil, fmt.Errorf("web token is required")
	}
	if options.PublicViews == nil {
		return nil, fmt.Errorf("public view reader is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if options.Mode == config.ModeProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	if err := engine.SetTrustedProxies(options.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	engine.Use(
		errorBoundary(logger),
		requestIDMiddleware(),
		securityHeadersMiddleware(),
		corsMiddleware(options.HTTP.CORS),
		bodyLimitMiddleware(options.HTTP.MaxBodyBytes, options.BodyLimitOverrides),
	)
	router := &Router{Engine: engine, API: newContractAPI(engine, logger)}

	health := options.Health
	if health == nil {
		health = NewHealth(nil, 0)
	}
	type pingInput struct{}
	type pingOutput struct {
		Body struct {
			Message string `json:"message"`
		}
	}
	Register(router.API, huma.Operation{
		OperationID: "ping",
		Method:      http.MethodGet,
		Path:        "/ping",
		Summary:     "Ping the API",
		Tags:        []string{"system"},
		Errors:      []int{http.StatusUnauthorized},
		Security:    []map[string][]string{{"webToken": {}}},
		Middlewares: huma.Middlewares{HumaWebAuthorization(options.WebToken)},
	}, func(context.Context, *pingInput) (*pingOutput, error) {
		output := &pingOutput{}
		output.Body.Message = "pong"
		return output, nil
	})
	registerPublicViewRoutes(router.API, options.WebToken, options.PublicViews)

	type healthInput struct{}
	type healthOutput struct {
		Status       int    `status:"204"`
		CacheControl string `header:"Cache-Control"`
	}
	healthMiddleware := huma.Middlewares{HumaBearerAuthorization(options.HealthcheckToken, "heyblog-health")}
	healthSecurity := []map[string][]string{{"healthBearer": {}}}
	Register(router.API, huma.Operation{
		OperationID:   "get-health-liveness",
		Method:        http.MethodGet,
		Path:          "/health/live",
		DefaultStatus: http.StatusNoContent,
		Summary:       "Check process liveness",
		Tags:          []string{"health"},
		Errors:        []int{http.StatusUnauthorized},
		Security:      healthSecurity,
		Middlewares:   healthMiddleware,
	}, func(context.Context, *healthInput) (*healthOutput, error) {
		return &healthOutput{Status: http.StatusNoContent, CacheControl: "no-store"}, nil
	})
	Register(router.API, huma.Operation{
		OperationID:   "get-health-readiness",
		Method:        http.MethodGet,
		Path:          "/health/ready",
		DefaultStatus: http.StatusNoContent,
		Summary:       "Check dependency readiness",
		Tags:          []string{"health"},
		Errors:        []int{http.StatusUnauthorized, http.StatusServiceUnavailable},
		Security:      healthSecurity,
		Middlewares:   healthMiddleware,
	}, func(ctx context.Context, _ *healthInput) (*healthOutput, error) {
		if err := health.Ready(ctx); err != nil {
			return nil, apperror.Wrap(
				err,
				apperror.KindUnavailable,
				apperror.CodeServiceUnavailable,
				"service is not ready",
				"check service readiness",
			)
		}
		return &healthOutput{Status: http.StatusNoContent, CacheControl: "no-store"}, nil
	})

	router.NoRoute(Adapt(func(*Context) (Response, error) {
		return Response{}, apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "the requested resource was not found")
	}))
	router.NoMethod(Adapt(func(*Context) (Response, error) {
		return Response{}, apperror.New(apperror.KindMethodNotAllowed, apperror.CodeMethodNotAllowed, "the method is not allowed for this resource")
	}))
	registerOpenAPIRoutes(router, options.Mode)
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
