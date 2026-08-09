package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"heyblog-api/internal/config"
)

const maximumReadinessProbeTimeout = 4 * time.Second

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func probeReadiness(ctx context.Context, configuration config.Config, client httpDoer) error {
	host := configuration.Server.Host
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() == nil {
			host = "::1"
		} else {
			host = "127.0.0.1"
		}
	}
	endpoint := "http://" + net.JoinHostPort(host, strconv.Itoa(configuration.Server.Port)) + "/health/ready"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create readiness probe: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+configuration.HealthcheckToken)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("execute readiness probe: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("readiness probe returned status %d", response.StatusCode)
	}
	return nil
}

func readinessProbeTimeout(configuration config.Config) time.Duration {
	timeout := configuration.Health.ReadinessTimeout + time.Second
	if timeout > maximumReadinessProbeTimeout {
		return maximumReadinessProbeTimeout
	}
	return timeout
}
