package sitemanagement

import (
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/auth"
)

func TestAccountHandler_when_SessionIsMissing(t *testing.T) {
	// Given an unauthenticated request.
	service := NewService(nil, testAuthentication{err: &auth.AuthError{StatusCode: 401, Code: "unauthorized", Message: "Authentication is required"}}, nil)
	// When the private account list is requested.
	_, err := listHandler(service, false)(t.Context(), &listRequest{})
	// Then authentication fails before accessing claims.
	require.Error(t, err)
}
func TestClaimDTO_when_ChallengeHasDigest(t *testing.T) {
	// Given an internal digest on the business claim.
	claim := Claim{ID: "claim", TokenHash: "secret-digest", SiteID: "internal-site"}
	// When it crosses the HTTP boundary.
	serialized, err := json.Marshal(claim)
	// Then internal verification material remains private.
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "secret-digest")
	require.NotContains(t, string(serialized), "internal-site")
}
func TestRegisterRoutes_when_ServiceAvailable(t *testing.T) {
	// Given the feature route registration surface.
	router := gin.New()
	configuration := huma.DefaultConfig("claims test", "1")
	configuration.OpenAPIPath = ""
	configuration.DocsPath = ""
	configuration.SchemasPath = ""
	api := humagin.New(router, configuration)
	// When claims and management routes are installed.
	err := RegisterRoutes(api, &Service{}, "test-token", nil)
	// Then all promised methods are available.
	require.NoError(t, err)
	expected := map[string]bool{"POST /account/site-claims": false, "GET /account/site-claims": false, "POST /account/site-claims/:claimId/check": false, "DELETE /account/site-claims/:claimId": false, "GET /management/site-claims": false, "GET /management/site-claims/:claimId": false, "POST /management/site-claims/:claimId/review": false, "DELETE /management/site-ownership/:shortId": false, "PUT /management/site-ownership/:shortId": false}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}
	for route, found := range expected {
		require.True(t, found, route)
	}
}
