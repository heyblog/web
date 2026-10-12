package sitemanagement

import (
	"encoding/json"
	"testing"
	"time"

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
	expected := map[string]bool{"POST /api/web/account/site-claims": false, "GET /api/web/account/site-claims": false, "POST /api/web/account/site-claims/:claimId/check": false, "DELETE /api/web/account/site-claims/:claimId": false, "GET /api/web/management/site-claims": false, "GET /api/web/management/site-claims/:claimId": false, "POST /api/web/management/site-claims/:claimId/review": false, "DELETE /api/web/management/site-ownership/:shortId": false, "GET /api/web/management/site-ownership/:shortId": false, "GET /api/web/account/sites/:shortId/claim-context": false, "POST /api/web/account/site-claims/:claimId/regenerate": false}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		require.NotEqual(t, "PUT /api/web/management/site-ownership/:shortId", key)
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}
	for route, found := range expected {
		require.True(t, found, route)
	}
}

func TestClaimMutationLimitsHaveSeparateBudgets(t *testing.T) {
	for _, test := range []struct {
		id, name string
		capacity int64
		interval time.Duration
	}{
		{"create-site-claim", "site-claims-create", 10, time.Hour},
		{"regenerate-site-claim", "site-claims-create", 10, time.Hour},
		{"check-site-claim", "site-claims-check", 30, time.Minute},
		{"cancel-site-claim", "site-claims-cancel", 30, time.Hour},
		{"review-site-claim", "site-claims-review", 60, time.Hour},
		{"revoke-site-ownership", "site-claims-review", 60, time.Hour},
	} {
		t.Run(test.id, func(t *testing.T) {
			policy := claimRatePolicy(test.id)
			require.Equal(t, test.name, policy.Name)
			require.Equal(t, test.capacity, policy.Capacity)
			require.Equal(t, test.interval, policy.RefillInterval)
		})
	}
}
