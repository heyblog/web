package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// EdgeConfig separates service authentication from target request signing.
type EdgeConfig struct {
	WorkerToken     string
	MasterKey       []byte
	CloudflareURL   string
	CloudflareToken string
	EdgeOneURL      string
	EdgeOneToken    string
}

func resolveEdge(getenv getenvFunc) (EdgeConfig, error) {
	c := EdgeConfig{WorkerToken: getenv("API_TASK_WORKER_TOKEN"), CloudflareURL: getenv("API_EDGE_CF_URL"), CloudflareToken: getenv("API_EDGE_CF_TOKEN"), EdgeOneURL: getenv("API_EDGE_EO_URL"), EdgeOneToken: getenv("API_EDGE_EO_TOKEN")}
	raw := getenv("API_REQUEST_SIGNING_MASTER_KEY")
	if raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return c, fmt.Errorf("API_REQUEST_SIGNING_MASTER_KEY must encode exactly 32 bytes in base64")
		}
		c.MasterKey = key
	}
	for name, value := range map[string]string{"API_TASK_WORKER_TOKEN": c.WorkerToken, "API_EDGE_CF_TOKEN": c.CloudflareToken, "API_EDGE_EO_TOKEN": c.EdgeOneToken} {
		if value != "" && (len(value) < 32 || strings.ContainsAny(value, " \r\n\t")) {
			return c, fmt.Errorf("%s must be at least 32 non-whitespace characters", name)
		}
	}
	for name, value := range map[string]string{"API_EDGE_CF_URL": c.CloudflareURL, "API_EDGE_EO_URL": c.EdgeOneURL} {
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return c, fmt.Errorf("%s must be an HTTPS endpoint without credentials, query or fragment", name)
		}
	}
	if (c.CloudflareURL == "") != (c.CloudflareToken == "") || (c.EdgeOneURL == "") != (c.EdgeOneToken == "") {
		return c, fmt.Errorf("each Edge endpoint requires its service token")
	}
	return c, nil
}

type WorkerConfig struct {
	APIURL string
	Token  string
}

func LoadWorker() (WorkerConfig, error) { return resolveWorker(os.Getenv) }
func resolveWorker(getenv getenvFunc) (WorkerConfig, error) {
	c := WorkerConfig{APIURL: strings.TrimRight(getenv("TASK_WORKER_API_URL"), "/"), Token: getenv("API_TASK_WORKER_TOKEN")}
	parsed, err := url.Parse(c.APIURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return c, fmt.Errorf("TASK_WORKER_API_URL must be an HTTP(S) API origin")
	}
	if parsed.Path != "" || len(c.Token) < 32 || strings.ContainsAny(c.Token, " \r\n\t") {
		return c, fmt.Errorf("worker API origin or token is invalid")
	}
	return c, nil
}
