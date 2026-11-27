package edgerequest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func (c *Client) dispatch(ctx context.Context, wire wireRequest) (wireResponse, error) {
	var response wireResponse
	endpoint, ok := c.config.Endpoints[wire.Platform]
	if !ok || endpoint.URL == "" || endpoint.Token == "" {
		return response, &Error{Kind: Platform, Cause: errors.New("edge platform is not configured")}
	}
	// Encode HTTP bytes as Latin-1 code points so JSON cannot replace invalid UTF-8
	// and Fetch receives the exact bytes of both configured text and opaque validators.
	wire.HeaderEncoding = "latin1"
	for name, value := range wire.Headers {
		wire.Headers[name] = encodeHeaderBytes(value)
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return response, &Error{Kind: Platform, Cause: err}
	}
	if len(body) > 256<<10 {
		return response, &Error{Kind: Security, Cause: errors.New("edge request envelope exceeds limit")}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return response, &Error{Kind: Platform, Cause: err}
	}
	request.Header.Set("Authorization", "Bearer "+endpoint.Token)
	request.Header.Set("Content-Type", "application/json")
	upstream, err := c.config.HTTPClient.Do(request)
	if err != nil {
		kind := Platform
		if ctx.Err() != nil {
			kind = Cancelled
		}
		return response, &Error{Kind: kind, Cause: err}
	}
	defer func() { _ = upstream.Body.Close() }()
	if upstream.StatusCode != http.StatusOK {
		response.Status = upstream.StatusCode
		response.Headers = map[string]string{"Retry-After": upstream.Header.Get("Retry-After")}
		return response, &Error{Kind: Platform, Cause: errors.New("edge service rejected request")}
	}
	// JSON base64 overhead plus bounded response metadata.
	limit := wire.MaxBytes*2 + 65536
	encoded, err := io.ReadAll(io.LimitReader(upstream.Body, limit+1))
	if err != nil {
		if ctx.Err() != nil {
			return response, &Error{Kind: Cancelled, Cause: ctx.Err()}
		}
		return response, &Error{Kind: Platform, Cause: err}
	}
	if int64(len(encoded)) > limit {
		return response, &Error{Kind: Platform, Cause: errors.New("invalid edge response size")}
	}
	if json.Unmarshal(encoded, &response) != nil {
		return response, &Error{Kind: Platform, Cause: errors.New("invalid edge response")}
	}
	for name, value := range response.Headers {
		decoded, err := decodeHeaderBytes(value)
		if err != nil {
			return response, err
		}
		response.Headers[name] = decoded
	}
	if response.Error != "" {
		kind := Platform
		switch response.Error {
		case "target_network", "target_timeout", "response_limit":
			kind = Target
		case "unsafe_target":
			kind = Security
		}
		return response, &Error{Kind: kind, Cause: errors.New("edge execution failed: " + response.Error)}
	}
	if response.Status < 100 || response.Status > 599 || int64(len(response.Body)) > wire.MaxBytes {
		return response, &Error{Kind: Platform, Cause: errors.New("invalid edge target response")}
	}
	return response, nil
}
