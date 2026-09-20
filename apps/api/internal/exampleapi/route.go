package exampleapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/apikey"
	"heyblog-api/internal/httpapi"
)

const (
	Path           = "/v1/example"
	AllowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
)

type emptyInput struct{}

type responseOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         responseBody
}

type responseBody struct {
	Method string `json:"method"`
	Status string `json:"status" enum:"ok"`
}

type noContentOutput struct {
	Status       int    `status:"204"`
	CacheControl string `header:"Cache-Control"`
}

type optionsOutput struct {
	Status       int    `status:"204"`
	CacheControl string `header:"Cache-Control"`
	Allow        string `header:"Allow"`
}

func RegisterRoutes(api huma.API, authenticator apikey.Authenticator) {
	for _, route := range []struct {
		id      string
		method  string
		summary string
	}{
		{id: "get-example", method: http.MethodGet, summary: "Call the GET example"},
		{id: "post-example", method: http.MethodPost, summary: "Call the POST example"},
		{id: "put-example", method: http.MethodPut, summary: "Call the PUT example"},
		{id: "patch-example", method: http.MethodPatch, summary: "Call the PATCH example"},
	} {
		httpapi.Register(api, operation(route.id, route.method, route.summary, http.StatusOK, authenticator), func(_ context.Context, _ *emptyInput) (*responseOutput, error) {
			return &responseOutput{
				CacheControl: "no-store",
				Body:         responseBody{Method: route.method, Status: "ok"},
			}, nil
		})
	}
	for _, route := range []struct {
		id      string
		method  string
		summary string
	}{
		{id: "head-example", method: http.MethodHead, summary: "Call the HEAD example"},
		{id: "delete-example", method: http.MethodDelete, summary: "Call the DELETE example"},
	} {
		httpapi.Register(api, operation(route.id, route.method, route.summary, http.StatusNoContent, authenticator), func(_ context.Context, _ *emptyInput) (*noContentOutput, error) {
			return &noContentOutput{Status: http.StatusNoContent, CacheControl: "no-store"}, nil
		})
	}
	httpapi.Register(api, operation("options-example", http.MethodOptions, "List example methods", http.StatusNoContent, authenticator), func(_ context.Context, _ *emptyInput) (*optionsOutput, error) {
		return &optionsOutput{
			Status: http.StatusNoContent, CacheControl: "no-store", Allow: AllowedMethods,
		}, nil
	})
}

func operation(
	id string,
	method string,
	summary string,
	status int,
	authenticator apikey.Authenticator,
) huma.Operation {
	return huma.Operation{
		OperationID:   id,
		Method:        method,
		Path:          Path,
		Summary:       summary,
		Tags:          []string{"example"},
		DefaultStatus: status,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden},
		Security:      []map[string][]string{{"apiBearer": {string(apikey.ScopeExampleCall)}}},
		Middlewares: huma.Middlewares{
			apikey.HumaAuthorization(authenticator, apikey.AccessPolicy{
				Audiences: []apikey.Audience{apikey.AudienceInternal, apikey.AudienceExternal}, Scope: apikey.ScopeExampleCall,
			}),
		},
	}
}
