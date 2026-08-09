package bootstrap

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProbeReadinessUsesResolvedServerPort(t *testing.T) {
	t.Parallel()

	configuration := applicationTestConfig()
	configuration.Server.Host = "0.0.0.0"
	configuration.Server.Port = 9432
	var requestedURL string
	var authorization string
	client := httpDoerFunc(func(request *http.Request) (*http.Response, error) {
		requestedURL = request.URL.String()
		authorization = request.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := probeReadiness(ctx, configuration, client); err != nil {
		t.Fatalf("probeReadiness() error = %v", err)
	}
	if requestedURL != "http://127.0.0.1:9432/health/ready" {
		t.Fatalf("URL = %q, want resolved configured port", requestedURL)
	}
	if authorization != "Bearer "+configuration.HealthcheckToken {
		t.Fatalf("Authorization = %q, want configured Bearer token", authorization)
	}
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (do httpDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return do(request)
}

func TestReadinessProbeTimeoutIsBounded(t *testing.T) {
	t.Parallel()

	configuration := applicationTestConfig()
	configuration.Health.ReadinessTimeout = time.Hour
	if got := readinessProbeTimeout(configuration); got != maximumReadinessProbeTimeout {
		t.Fatalf("timeout = %s, want %s", got, maximumReadinessProbeTimeout)
	}
}
