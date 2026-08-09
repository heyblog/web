package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"heyblog-api/internal/config"
	"heyblog-api/internal/httpapi"
)

func TestRunClosesDependenciesWhenListenFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("address in use")
	var closeCount atomic.Int32
	err := run(context.Background(), applicationTestConfig(), discardLogger(), applicationOperations{
		listen: func(string, string) (net.Listener, error) { return nil, wantErr },
		openDependencies: func(context.Context, config.Config) (runtimeDependencies, error) {
			return &stubRuntimeDependencies{close: func() error {
				closeCount.Add(1)
				return nil
			}}, nil
		},
		newHandler: func(httpapi.Options) (http.Handler, error) { return http.NewServeMux(), nil },
		newServer:  func(http.Handler) managedHTTPServer { return &stubHTTPServer{} },
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("run() error = %v, want listen error", err)
	}
	if closeCount.Load() != 1 {
		t.Fatalf("dependency close count = %d, want 1", closeCount.Load())
	}
}

func TestRunDoesNotListenWhenDependenciesFail(t *testing.T) {
	t.Parallel()

	listenCalled := false
	wantErr := errors.New("migration failed")
	err := run(context.Background(), applicationTestConfig(), discardLogger(), applicationOperations{
		listen: func(string, string) (net.Listener, error) {
			listenCalled = true
			return &stubListener{}, nil
		},
		openDependencies: func(context.Context, config.Config) (runtimeDependencies, error) {
			return nil, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("run() error = %v, want dependency error", err)
	}
	if listenCalled {
		t.Fatal("listener opened before dependencies were ready")
	}
}

func TestRunForcesServerCloseBeforeDependenciesOnShutdownFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	shutdownErr := errors.New("shutdown deadline")
	var sequence atomic.Int32
	var shutdownOrder atomic.Int32
	var forceCloseOrder atomic.Int32
	var dependencyCloseOrder atomic.Int32
	server := &stubHTTPServer{
		serveDone: make(chan struct{}),
		shutdown: func(context.Context) error {
			shutdownOrder.Store(sequence.Add(1))
			return shutdownErr
		},
		close: func() error {
			forceCloseOrder.Store(sequence.Add(1))
			return nil
		},
	}
	dependencies := &stubRuntimeDependencies{close: func() error {
		dependencyCloseOrder.Store(sequence.Add(1))
		return nil
	}}
	var health *httpapi.Health
	var healthcheckToken string
	err := run(ctx, applicationTestConfig(), discardLogger(), applicationOperations{
		listen:           func(string, string) (net.Listener, error) { return &stubListener{}, nil },
		openDependencies: func(context.Context, config.Config) (runtimeDependencies, error) { return dependencies, nil },
		newHandler: func(options httpapi.Options) (http.Handler, error) {
			health = options.Health
			healthcheckToken = options.HealthcheckToken
			return http.NewServeMux(), nil
		},
		newServer: func(http.Handler) managedHTTPServer { return server },
	})

	if !errors.Is(err, shutdownErr) {
		t.Fatalf("run() error = %v, want shutdown error", err)
	}
	if shutdownOrder.Load() == 0 || forceCloseOrder.Load() <= shutdownOrder.Load() || dependencyCloseOrder.Load() <= forceCloseOrder.Load() {
		t.Fatalf("orders = shutdown:%d force:%d dependencies:%d, want strict shutdown order", shutdownOrder.Load(), forceCloseOrder.Load(), dependencyCloseOrder.Load())
	}
	if health == nil || health.Ready(context.Background()) == nil {
		t.Fatal("health was not marked draining before shutdown")
	}
	if healthcheckToken != applicationTestConfig().HealthcheckToken {
		t.Fatalf("healthcheck token = %q, want configured token", healthcheckToken)
	}
}

func TestHTTPServerBaseContextOutlivesProcessCancellation(t *testing.T) {
	t.Parallel()

	processContext, cancel := context.WithCancel(context.Background())
	server := newHTTPServer(processContext, applicationTestConfig(), http.NewServeMux(), discardLogger())
	requestBase := server.BaseContext(&stubListener{})
	cancel()

	select {
	case <-requestBase.Done():
		t.Fatal("request base context was canceled by the process signal")
	default:
	}
}

func applicationTestConfig() config.Config {
	return config.Config{
		Mode:             config.ModeDevelopment,
		HealthcheckToken: "test-healthcheck-token-0123456789abcdef",
		Server:           config.ServerConfig{Host: "127.0.0.1", Port: 10201},
		HTTP: config.HTTPConfig{
			ReadHeaderTimeout: time.Second,
			ReadTimeout:       time.Second,
			WriteTimeout:      time.Second,
			IdleTimeout:       time.Second,
			ShutdownTimeout:   time.Millisecond,
			MaxHeaderBytes:    1024,
			MaxBodyBytes:      1024,
		},
		Health: config.HealthConfig{ReadinessTimeout: time.Second, DrainDelay: time.Millisecond},
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubListener struct {
	closeCount atomic.Int32
}

func (listener *stubListener) Accept() (net.Conn, error) { return nil, errors.New("not implemented") }
func (listener *stubListener) Close() error {
	listener.closeCount.Add(1)
	return nil
}
func (listener *stubListener) Addr() net.Addr { return stubAddress("test") }

type stubAddress string

func (address stubAddress) Network() string { return string(address) }
func (address stubAddress) String() string  { return string(address) }

type stubRuntimeDependencies struct {
	close func() error
}

func (*stubRuntimeDependencies) Ready(context.Context) error { return nil }
func (dependencies *stubRuntimeDependencies) Close() error   { return dependencies.close() }

type stubHTTPServer struct {
	serveDone chan struct{}
	shutdown  func(context.Context) error
	close     func() error
}

func (server *stubHTTPServer) Serve(net.Listener) error {
	<-server.serveDone
	return http.ErrServerClosed
}
func (server *stubHTTPServer) Shutdown(ctx context.Context) error { return server.shutdown(ctx) }
func (server *stubHTTPServer) Close() error {
	select {
	case <-server.serveDone:
	default:
		close(server.serveDone)
	}
	return server.close()
}
