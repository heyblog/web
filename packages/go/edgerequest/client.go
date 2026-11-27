package edgerequest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

func New(config Config) *Client {
	if config.Resolver == nil {
		config.Resolver = net.DefaultResolver
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}
	// Edge service redirects must never carry its bearer token to another host.
	client := *config.HTTPClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	config.HTTPClient = &client
	return &Client{config: config}
}
func (c *Client) Execute(ctx context.Context, request Request) (result Result, err error) {
	result.Platform = request.Platform
	endpoint, configured := c.config.Endpoints[request.Platform]
	if !configured || endpoint.URL == "" || endpoint.Token == "" {
		return result, &Error{Kind: Platform, Cause: errors.New("edge platform is not configured")}
	}
	if request.Method != "GET" && request.Method != "HEAD" || request.MaxBytes < 1 || request.MaxBytes > 4<<20 || request.Timeout < time.Millisecond || request.Timeout > 60*time.Second || request.MaxRedirects < 0 || request.MaxRedirects > 10 || (request.Platform != Cloudflare && request.Platform != EdgeOne) {
		return result, &Error{Kind: Security, Cause: errors.New("invalid request limits")}
	}
	started := time.Now()
	defer func() { result.Duration = time.Since(started) }()
	// Authorization can wait for task pacing. Keep that wait inside the total
	// operation deadline, but outside the cumulative DNS/transport budget.
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	budget := networkBudget{remaining: request.Timeout}
	var original *url.URL
	current := request.URL
	for {
		if ctx.Err() != nil {
			return result, &Error{Kind: Cancelled, Cause: ctx.Err()}
		}
		network, finish := budget.start(ctx)
		target, ip, err := c.target(network, current)
		finish()
		if err != nil {
			return result, err
		}
		current = target.String()
		if len(result.Redirects) > 0 {
			result.Redirects[len(result.Redirects)-1] = current
		}
		if original == nil {
			original = target
		}
		if request.SameOrigin && (target.Scheme != original.Scheme || target.Host != original.Host) {
			return result, &Error{Kind: Security, Cause: errors.New("redirect changed origin")}
		}
		headers, err := SafeHeaders(request.Headers)
		if err != nil {
			return result, err
		}
		if c.config.Authorizer == nil {
			return result, &Error{Kind: Signature, Cause: errors.New("request authorizer is unavailable")}
		}
		headers, err = c.config.Authorizer.Authorize(ctx, request.Platform, request.Method, current, headers)
		if err != nil {
			if ctx.Err() != nil {
				return result, &Error{Kind: Cancelled, Cause: ctx.Err()}
			}
			var failure *Error
			if errors.As(err, &failure) {
				return result, err
			}
			return result, &Error{Kind: Signature, Cause: err}
		}
		headers, err = SafeHeaders(headers)
		if err != nil {
			return result, err
		}
		if c.config.BeforeDispatch != nil {
			if err := c.config.BeforeDispatch(ctx, request.Platform); err != nil {
				var failure *Error
				if errors.As(err, &failure) {
					return result, err
				}
				return result, &Error{Kind: Backoff, Cause: err}
			}
		}
		network, finish = budget.start(ctx)
		if err := network.Err(); err != nil {
			finish()
			return result, &Error{Kind: Cancelled, Cause: err}
		}
		deadline, _ := network.Deadline()
		remaining := time.Until(deadline)
		// The Edge timer starts after service transport and request parsing. Leave
		// bounded return time for its explicit target_timeout result; transport
		// expiry itself remains indeterminate and never establishes target failure.
		returnAllowance := min(250*time.Millisecond, remaining/4)
		targetTimeoutMS := max(int64(1), (remaining - returnAllowance).Milliseconds())
		wire := wireRequest{URL: current, Method: request.Method, Platform: request.Platform, Headers: map[string]string{}, MaxBytes: request.MaxBytes, TimeoutMS: targetTimeoutMS, ResolvedIP: ip}
		for name, values := range headers {
			wire.Headers[name] = values[0]
		}
		result.Requests++
		response, err := c.dispatch(network, wire)
		finish()
		if c.config.Record != nil {
			observation := Observation{URL: current, Method: request.Method, Platform: request.Platform,
				Result: Result{Status: response.Status, Headers: make(http.Header), Body: response.Body,
					Platform: request.Platform, Node: response.Node, Duration: time.Since(started), FinalURL: current}}
			for name, value := range response.Headers {
				observation.Result.Headers.Set(name, value)
			}
			var failure *Error
			if errors.As(err, &failure) {
				observation.Error = failure.Kind
			}
			if recordErr := c.config.Record(ctx, observation); recordErr != nil {
				return result, &Error{Kind: Platform, Cause: recordErr}
			}
		}
		if err != nil {
			return result, err
		}
		result.Status = response.Status
		result.Body = response.Body
		result.Node = response.Node
		result.FinalURL = current
		result.Headers = make(http.Header)
		for key, value := range response.Headers {
			result.Headers.Set(key, value)
		}
		switch response.Status {
		case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
			if len(result.Redirects) >= request.MaxRedirects {
				return result, &Error{Kind: Target, Cause: errors.New("redirect limit exceeded")}
			}
			location, err := target.Parse(result.Headers.Get("Location"))
			if err != nil || result.Headers.Get("Location") == "" {
				return result, &Error{Kind: Target, Cause: errors.New("invalid target redirect")}
			}
			location.Fragment = ""
			current = location.String()
			result.Redirects = append(result.Redirects, current)
		default:
			return result, nil
		}
	}
}
