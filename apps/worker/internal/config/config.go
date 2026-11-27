package config

import (
	"errors"
	"net/url"
	"os"
	"strings"

	"heyblog.local/edgerequest"
)

var ErrConfiguration = errors.New("invalid worker configuration")

type Config struct {
	APIURL    string
	Token     string
	Health    string
	Endpoints map[edgerequest.Provider]edgerequest.Endpoint
}

func Load() (Config, error) { return Parse(os.Getenv) }

func Parse(getenv func(string) string) (Config, error) {
	c := Config{APIURL: strings.TrimRight(getenv("TASK_WORKER_API_URL"), "/"), Token: getenv("API_TASK_WORKER_TOKEN"),
		Health: "127.0.0.1:10202", Endpoints: map[edgerequest.Provider]edgerequest.Endpoint{}}
	u, err := url.Parse(c.APIURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" ||
		(u.Scheme != "http" && u.Scheme != "https") || len(c.Token) < 32 || strings.ContainsAny(c.Token, " \r\n\t") {
		return c, ErrConfiguration
	}
	for _, platform := range []edgerequest.Provider{edgerequest.Cloudflare, edgerequest.EdgeOne} {
		prefix := "API_EDGE_" + string(platform)
		endpoint := edgerequest.Endpoint{URL: getenv(prefix + "_URL"), Token: getenv(prefix + "_TOKEN")}
		if endpoint.URL == "" && endpoint.Token == "" {
			continue
		}
		u, err := url.Parse(endpoint.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
			len(endpoint.Token) < 32 || strings.ContainsAny(endpoint.Token, " \r\n\t") {
			return c, ErrConfiguration
		}
		c.Endpoints[platform] = endpoint
	}
	return c, nil
}
