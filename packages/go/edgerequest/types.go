// Package edgerequest owns bounded, independently authorized Edge requests.
package edgerequest

import (
	"context"
	"net/http"
	"net/netip"
	"time"
)

type Provider string

const (
	Cloudflare Provider = "CF"
	EdgeOne    Provider = "EO"
)

type Kind string

const (
	Target    Kind = "target"
	Platform  Kind = "platform"
	Security  Kind = "security"
	Signature Kind = "signature"
	Backoff   Kind = "backoff"
	DNS       Kind = "resolver"
	Cancelled Kind = "cancelled"
)

type Error struct {
	Kind  Kind
	Cause error
}

func (e *Error) Error() string { return "edge request: " + string(e.Kind) }
func (e *Error) Unwrap() error { return e.Cause }

type Endpoint struct {
	URL   string
	Token string
}
type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type Authorizer interface {
	Authorize(context.Context, Provider, string, string, http.Header) (http.Header, error)
}
type Config struct {
	Endpoints      map[Provider]Endpoint
	Resolver       Resolver
	Authorizer     Authorizer
	BeforeDispatch func(context.Context, Provider) error
	Record         func(context.Context, Observation) error
	HTTPClient     *http.Client
}
type Observation struct {
	URL      string
	Method   string
	Platform Provider
	Result   Result
	Error    Kind
}
type Client struct{ config Config }
type Request struct {
	URL          string
	Method       string
	Platform     Provider
	Headers      http.Header
	MaxBytes     int64
	Timeout      time.Duration
	MaxRedirects int
	SameOrigin   bool
}
type Result struct {
	Status    int
	Headers   http.Header
	Body      []byte
	Platform  Provider
	Node      map[string]string
	Requests  int
	Redirects []string
	Duration  time.Duration
	FinalURL  string
}
type wireRequest struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Platform       Provider          `json:"platform"`
	Headers        map[string]string `json:"headers"`
	HeaderEncoding string            `json:"headerEncoding"`
	MaxBytes       int64             `json:"maxBytes"`
	TimeoutMS      int64             `json:"timeoutMs"`
	ResolvedIP     string            `json:"resolvedIP"`
}
type wireResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
	Node    map[string]string `json:"node"`
	Error   string            `json:"error,omitempty"`
}
