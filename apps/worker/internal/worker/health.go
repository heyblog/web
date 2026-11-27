package worker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

func (w *Worker) ServeHealth(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(out http.ResponseWriter, _ *http.Request) {
		out.Header().Set("Cache-Control", "no-store")
		out.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /health/ready", func(out http.ResponseWriter, _ *http.Request) {
		out.Header().Set("Cache-Control", "no-store")
		if !w.ready.Load() {
			out.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		out.WriteHeader(http.StatusNoContent)
	})
	listener, err := net.Listen("tcp", w.config.Health)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 10 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer stop()
		if err := server.Shutdown(shutdown); err != nil {
			return err
		}
		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
