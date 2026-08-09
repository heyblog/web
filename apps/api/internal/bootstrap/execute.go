package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"heyblog-api/internal/config"
	"heyblog-api/internal/logging"
)

func Execute(args []string, stdout, stderr io.Writer) int {
	fallbackLogger := slog.New(slog.NewTextHandler(stderr, nil))
	healthcheck, err := parseInvocation(args)
	if err != nil {
		fallbackLogger.Error("API invocation failed",
			"event", "invocation_failed",
			"error_type", fmt.Sprintf("%T", err),
		)
		return 2
	}
	configuration, err := config.Load()
	if err != nil {
		fallbackLogger.Error("API configuration failed",
			"event", "configuration_failed",
			"error_type", fmt.Sprintf("%T", err),
			"error", err.Error(),
		)
		return 1
	}
	if healthcheck {
		probeContext, cancel := context.WithTimeout(context.Background(), readinessProbeTimeout(configuration))
		defer cancel()
		if err := probeReadiness(probeContext, configuration, &http.Client{}); err != nil {
			fallbackLogger.Error("API readiness probe failed",
				"event", "readiness_probe_failed",
				"error_type", fmt.Sprintf("%T", err),
			)
			return 1
		}
		return 0
	}

	logRuntime, err := logging.New(configuration.Mode, configuration.Logging, stdout)
	if err != nil {
		fallbackLogger.Error("API logging initialization failed",
			"event", "logging_initialization_failed",
			"error_type", fmt.Sprintf("%T", err),
		)
		return 1
	}
	logger := logRuntime.Logger

	processContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runErr := Run(processContext, configuration, logger)
	if runErr != nil {
		stages, causeTypes := failureMetadata(runErr)
		logger.Error("API stopped with an error",
			"event", "application_failed",
			"failure_stages", stages,
			"cause_types", causeTypes,
		)
	}
	if closeErr := logRuntime.Close(); closeErr != nil {
		fallbackLogger.Error("API logger close failed",
			"event", "logging_close_failed",
			"error_type", fmt.Sprintf("%T", closeErr),
		)
		if runErr == nil {
			runErr = closeErr
		}
	}
	if runErr != nil {
		return 1
	}
	return 0
}

func parseInvocation(args []string) (bool, error) {
	switch {
	case len(args) == 0:
		return false, nil
	case len(args) == 1 && args[0] == "--healthcheck":
		return true, nil
	default:
		return false, fmt.Errorf("only --healthcheck is supported")
	}
}
