//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/taskcenter"
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"

	"github.com/stretchr/testify/require"
)

type taskRouteActor struct{ user auth.User }

func (a taskRouteActor) Current(context.Context, *http.Request) (auth.User, error) {
	return a.user, nil
}

func TestSigningMutationsReplayReceiptsWithoutRepeatingChanges(t *testing.T) {
	// Given an authenticated administrator and the real signing persistence boundary.
	h := newTaskHarness(t, "monitor")
	var actor string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
		VALUES('signing-routes@example.test','signing_routes','Signing','SYS_ADMIN') RETURNING id::text`).Scan(&actor))
	router, err := httpapi.NewRouter(httpapi.Options{WebToken: "web-token", HealthcheckToken: "health-token", HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}})
	require.NoError(t, err)
	taskcenter.RegisterRoutes(router.API, taskcenter.Routes{Repository: h.repo, Signing: h.signing,
		Auth: taskRouteActor{auth.User{ID: actor, Role: auth.RoleSysAdmin}}, WebToken: "web-token", WorkerToken: "worker-token"})
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/web/management/request-signing/"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(httpapi.WebTokenHeader, "web-token")
		req.Header.Set("Idempotency-Key", key)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	// When a generated key response is replayed and its key is reused with a different payload.
	first := request("POST", "keys", `{"platform":"CF"}`, "generate")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	replay := request("POST", "keys", `{"platform":"CF"}`, "generate")
	require.Equal(t, http.StatusOK, replay.Code, replay.Body.String())
	require.JSONEq(t, first.Body.String(), replay.Body.String())
	require.Equal(t, http.StatusConflict, request("POST", "keys", `{"platform":"EO"}`, "generate").Code)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &created))
	for _, action := range []string{"publish", "activate", "revoke"} {
		for range 2 {
			result := request("POST", "keys/"+created.ID+"/"+action, `{}`, action)
			require.Equal(t, http.StatusOK, result.Code, result.Body.String())
		}
	}
	policy, err := h.signing.Policy(t.Context())
	require.NoError(t, err)
	policy.Carrier = "header"
	body, err := json.Marshal(policy)
	require.NoError(t, err)
	saved := request("PUT", "policy", string(body), "policy")
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	replayed := request("PUT", "policy", string(body), "policy")
	require.Equal(t, http.StatusOK, replayed.Code, replayed.Body.String())
	require.JSONEq(t, saved.Body.String(), replayed.Body.String())
	var returned taskcenter.Policy
	require.NoError(t, json.Unmarshal(saved.Body.Bytes(), &returned))
	// Then every key/transition/policy mutation is durable exactly once, including its audit event.
	require.Equal(t, policy.Revision+1, returned.Revision)
	keys, err := h.signing.Keys(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "revoked", keys[0].State)
	events, err := h.signing.Events(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 5)
}
