package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := serve(); err != nil {
		log.Fatal(err)
	}
}

func serve() error {
	secret := os.Getenv("DEPLOYER_NOTIFY_SECRET")
	if len(secret) < 32 {
		return fmt.Errorf("DEPLOYER_NOTIFY_SECRET must be at least 32 bytes")
	}
	// The signing secret belongs to the HTTP service, not to Docker or its wrapper.
	if err := os.Unsetenv("DEPLOYER_NOTIFY_SECRET"); err != nil {
		return err
	}
	composeFile := os.Getenv("DEPLOYER_COMPOSE_FILE")
	envFile := os.Getenv("DEPLOYER_ENV_FILE")
	stateDir := os.Getenv("DEPLOYER_STATE_DIR")
	for name, path := range map[string]string{
		"DEPLOYER_COMPOSE_FILE": composeFile,
		"DEPLOYER_ENV_FILE":     envFile,
		"DEPLOYER_STATE_DIR":    stateDir,
	} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s must be an absolute path", name)
		}
	}
	liveURL, err := url.Parse(envDefault("DEPLOYER_LIVE_URL", "https://www.heyblog.net/"))
	if err != nil || liveURL.Scheme != "https" || liveURL.Hostname() == "" || liveURL.User != nil {
		return fmt.Errorf("DEPLOYER_LIVE_URL must be an HTTPS URL")
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	// Put the agent on the legacy loopback port for use behind the host HTTPS proxy.
	addr := net.JoinHostPort("127.0.0.1", envDefault("DEPLOYER_PORT", "9401"))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	host := newComposeHost(envDefault("DEPLOYER_DOCKER_BIN", "docker"), composeFile, envFile, liveURL)
	deployer := &deployer{secret: secret, stateDir: stateDir, host: host}
	if err := deployer.recover(); err != nil {
		return err
	}
	server := &http.Server{
		Handler:           deployer.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return server.Serve(listener)
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
