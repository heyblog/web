package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"heyblog-worker/internal/config"
	"heyblog-worker/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if len(os.Args) == 2 && os.Args[1] == "--healthcheck" {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://127.0.0.1:10202/health/ready")
		if err != nil {
			os.Exit(1)
		}
		if err := response.Body.Close(); err != nil {
			os.Exit(1)
		}
		if response.StatusCode != http.StatusNoContent {
			os.Exit(1)
		}
		return
	}
	settings, err := config.Load()
	if err != nil {
		logger.Error("worker configuration invalid", "event", "worker_configuration_failed")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	service := worker.New(settings, logger)
	healthCtx, healthStop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		err := service.ServeHealth(healthCtx)
		if err != nil {
			stop()
		}
		done <- err
	}()
	service.Run(ctx)
	healthStop()
	if err := <-done; err != nil {
		logger.Error("worker health server failed", "event", "worker_health_failed")
	}
}
