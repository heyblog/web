package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/auth"
	"heyblog-api/internal/config"
	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/mail"
	"heyblog-api/internal/temp/dataimport"
)

type runtimeDependencies interface {
	httpapi.Readiness
	PublicViews() publicview.Reader
	DatabasePool() *pgxpool.Pool
	RedisClient() *redis.Client
	Mail() mail.Sender
	Verification() *mail.VerificationMailer
	Close() error
}

type managedHTTPServer interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
	Close() error
}

type applicationOperations struct {
	listen           func(string, string) (net.Listener, error)
	openDependencies func(context.Context, config.Config) (runtimeDependencies, error)
	newHandler       func(httpapi.Options, runtimeDependencies, config.Config, string) (http.Handler, error)
	newServer        func(http.Handler) managedHTTPServer
}

func Run(ctx context.Context, configuration config.Config, logger *slog.Logger) error {
	return run(ctx, configuration, logger, applicationOperations{
		listen: net.Listen,
		openDependencies: func(ctx context.Context, configuration config.Config) (runtimeDependencies, error) {
			return Open(ctx, configuration)
		},
		newHandler: func(options httpapi.Options, dependencies runtimeDependencies, configuration config.Config, importToken string) (http.Handler, error) {
			return newApplicationHandler(options, dependencies, configuration, importToken)
		},
		newServer: func(handler http.Handler) managedHTTPServer {
			return newHTTPServer(ctx, configuration, handler, logger)
		},
	})
}

func run(ctx context.Context, configuration config.Config, logger *slog.Logger, operations applicationOperations) (resultErr error) {
	dependencies, err := operations.openDependencies(ctx, configuration)
	if err != nil {
		return withStage("dependencies_open", err)
	}
	defer func() {
		if err := dependencies.Close(); err != nil {
			resultErr = errors.Join(resultErr, withStage("dependencies_close", err))
		}
	}()
	logger.InfoContext(ctx, "application dependencies ready", "event", "dependencies_ready")

	health := httpapi.NewHealth(dependencies, configuration.Health.ReadinessTimeout)
	handler, err := operations.newHandler(httpapi.Options{
		Mode:             configuration.Mode,
		HTTP:             configuration.HTTP,
		Logger:           logger,
		Health:           health,
		HealthcheckToken: configuration.HealthcheckToken,
		WebToken:         configuration.WebToken,
		PublicViews:      dependencies.PublicViews(),
	}, dependencies, configuration, configuration.TempImportToken)
	if err != nil {
		return withStage("router_build", err)
	}
	server := operations.newServer(handler)

	listener, err := operations.listen("tcp", configuration.ListenAddress())
	if err != nil {
		return withStage("listener_bind", err)
	}
	listenerOwnedByServer := false
	defer func() {
		if !listenerOwnedByServer {
			resultErr = errors.Join(resultErr, withStage("listener_close", listener.Close()))
		}
	}()
	logger.InfoContext(ctx, "HTTP listener bound",
		"event", "listener_bound",
		"address", configuration.ListenAddress(),
	)

	listenerOwnedByServer = true
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()
	logger.InfoContext(ctx, "API server started",
		"event", "server_started",
		"address", configuration.ListenAddress(),
	)

	select {
	case serveErr := <-serverErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return withStage("http_serve", serveErr)
		}
		return nil
	case <-ctx.Done():
		health.BeginDrain()
		logger.Info("API server draining",
			"event", "server_draining",
			"drain_delay_ms", configuration.Health.DrainDelay.Milliseconds(),
		)
	}
	if configuration.Health.DrainDelay > 0 {
		drainTimer := time.NewTimer(configuration.Health.DrainDelay)
		select {
		case <-drainTimer.C:
		case serveErr := <-serverErrors:
			drainTimer.Stop()
			if !errors.Is(serveErr, http.ErrServerClosed) {
				return withStage("http_drain", serveErr)
			}
			return nil
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.HTTP.ShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownContext)
	if shutdownErr != nil {
		logger.Error("graceful HTTP shutdown failed",
			"event", "shutdown_failed",
			"error_type", fmt.Sprintf("%T", shutdownErr),
		)
		closeErr := server.Close()
		resultErr = errors.Join(
			withStage("http_shutdown", shutdownErr),
			withStage("http_force_close", closeErr),
		)
	}
	if serveErr := <-serverErrors; !errors.Is(serveErr, http.ErrServerClosed) {
		resultErr = errors.Join(resultErr, withStage("http_shutdown_serve", serveErr))
	}
	logger.Info("API server stopped", "event", "server_stopped")
	return resultErr
}

func newApplicationHandler(options httpapi.Options, dependencies runtimeDependencies, configuration config.Config, importToken string) (http.Handler, error) {
	options.BodyLimitOverrides = dataimport.BodyLimitOverrides()
	router, err := httpapi.NewRouter(options)
	if err != nil {
		return nil, err
	}
	service := dataimport.NewService(dataimport.NewRepository(dependencies.DatabasePool()), site.NewShortID)
	dataimport.RegisterRoutes(router, service, importToken, options.Logger)
	authService := auth.NewService(auth.Dependencies{Pool: dependencies.DatabasePool(), Redis: dependencies.RedisClient(), MailSender: dependencies.Mail(), VerificationMailer: dependencies.Verification(), Config: auth.Config{
		AccessSecret: configuration.Auth.AccessSecret, RefreshSecret: configuration.Auth.RefreshSecret, AccessTTL: configuration.Auth.AccessTTL, RefreshTTL: configuration.Auth.RefreshTTL,
		VerificationTTL: configuration.Auth.VerificationTTL, PasswordResetTTL: configuration.Auth.PasswordResetTTL, WebBaseURL: configuration.Auth.WebBaseURL, CookieDomain: configuration.Auth.CookieDomain,
		GithubClientID: configuration.Auth.GithubClientID, GithubClientSecret: configuration.Auth.GithubClientSecret, GithubScope: configuration.Auth.GithubScope,
		MailFrom: configuration.Mail.Senders.Verification.Address,
	}})
	if err := auth.RegisterRoutes(router, authService, options.WebToken); err != nil {
		return nil, err
	}
	return router, nil
}

func newHTTPServer(processContext context.Context, configuration config.Config, handler http.Handler, logger *slog.Logger) *http.Server {
	requestBaseContext := context.WithoutCancel(processContext)
	return &http.Server{
		Addr:              configuration.ListenAddress(),
		Handler:           handler,
		ReadHeaderTimeout: configuration.HTTP.ReadHeaderTimeout,
		ReadTimeout:       configuration.HTTP.ReadTimeout,
		WriteTimeout:      configuration.HTTP.WriteTimeout,
		IdleTimeout:       configuration.HTTP.IdleTimeout,
		MaxHeaderBytes:    configuration.HTTP.MaxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		BaseContext: func(net.Listener) context.Context {
			return requestBaseContext
		},
	}
}
