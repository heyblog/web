//go:build integration

package integration_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"heyblog-api/internal/domain/proof"

	"heyblog.local/taskprotocol"

	edgeclient "heyblog.local/edgerequest"

	"github.com/stretchr/testify/require"
)

func TestRequestSigningCurrentPolicyRotationAndConcurrentRequests(t *testing.T) {
	// Given active signing material, without a request quota.
	h := newTaskHarness(t, "monitor")
	var actor string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
		VALUES('signing@example.test','signing_admin','Signing','SYS_ADMIN') RETURNING id::text`).Scan(&actor))
	_, err := h.fixture.pool.Exec(t.Context(), `UPDATE operations.request_configs SET parameters=jsonb_set(parameters,'{signed}','true')`)
	require.NoError(t, err)
	key, err := h.signing.Generate(t.Context(), actor, proof.Cloudflare)
	require.NoError(t, err)
	require.Error(t, h.signing.Transition(t.Context(), actor, key, "activate"))
	for _, action := range []string{"publish", "activate"} {
		require.NoError(t, h.signing.Transition(t.Context(), actor, key, action))
	}
	for i := range 8 {
		insertSite(t.Context(), t, h.fixture.pool, fmt.Sprintf("Sign%05d", i), "Signing", fmt.Sprintf("sign-%d.example.test", i))
	}
	_, err = h.fixture.pool.Exec(t.Context(), `UPDATE operations.schedules SET configuration=jsonb_set(configuration,'{concurrency}','8') WHERE id=$1`, h.schedule.ID)
	require.NoError(t, err)
	first := h.start(t)
	work := []*taskprotocol.Work{first}
	for range 7 {
		w, err := h.repo.Claim(t.Context(), h.worker)
		require.NoError(t, err)
		require.NotNil(t, w)
		work = append(work, w)
	}
	var wg sync.WaitGroup
	tokens := make(chan string, 8)
	// When concurrent authorized requests use the current live key.
	for _, w := range work {
		wg.Go(func() {
			permit, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, "CF", 0))
			if err != nil {
				t.Error(err)
				return
			}
			tokens <- string(permit.Headers["X-Heyblog-Proof"])
		})
	}
	wg.Wait()
	require.Len(t, tokens, 8)
	token := <-tokens
	check, err := h.signing.Verify(t.Context(), token)
	require.NoError(t, err)
	require.Equal(t, "valid", check.Status)
	next, err := h.signing.Generate(t.Context(), actor, proof.Cloudflare)
	require.NoError(t, err)
	for _, action := range []string{"publish", "activate"} {
		require.NoError(t, h.signing.Transition(t.Context(), actor, next, action))
	}
	check, err = h.signing.Verify(t.Context(), token)
	require.NoError(t, err)
	require.Equal(t, "valid", check.Status)
	require.NoError(t, h.signing.Transition(t.Context(), actor, key, "revoke"))
	check, err = h.signing.Verify(t.Context(), token)
	require.NoError(t, err)
	require.Equal(t, "revoked", check.Status)
	// Then disabling signing fails closed, rather than downgrading existing signed configurations.
	p, err := h.signing.Policy(t.Context())
	require.NoError(t, err)
	p.Enabled = false
	require.NoError(t, h.signing.SetPolicy(t.Context(), actor, p))
	_, err = h.signing.Permit(t.Context(), first.ID, h.authorization(first, "CF", 0))
	var failure *edgeclient.Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, edgeclient.Signature, failure.Kind)
}

func TestSigningRejectsOversizedFinalHeadersBeforeAuthorization(t *testing.T) {
	// Given a legal UA whose appended proof would exceed the header limit.
	h := newTaskHarness(t, "monitor")
	var actor string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role)
		VALUES('final-headers@example.test','final_headers','Signing','SYS_ADMIN') RETURNING id::text`).Scan(&actor))
	_, err := h.fixture.pool.Exec(t.Context(), `UPDATE operations.request_configs SET parameters=jsonb_set(parameters,'{signed}','true')`)
	require.NoError(t, err)
	key, err := h.signing.Generate(t.Context(), actor, proof.Cloudflare)
	require.NoError(t, err)
	for _, action := range []string{"publish", "activate"} {
		require.NoError(t, h.signing.Transition(t.Context(), actor, key, action))
	}
	_, err = h.fixture.pool.Exec(t.Context(), `UPDATE operations.request_configs SET parameters=jsonb_set(parameters,'{user_agent}',to_jsonb($1::text))`, strings.Repeat("A", 256))
	require.NoError(t, err)
	insertSite(t.Context(), t, h.fixture.pool, "LongHdr01", "Long headers", "headers.example.test")
	_, err = h.fixture.pool.Exec(t.Context(), `UPDATE directory.sites SET base_path=$1 WHERE short_id='LongHdr01'`, "/"+strings.Repeat("a", 5750))
	require.NoError(t, err)
	w := h.start(t)
	// When final proof-bearing headers exceed the bound.
	_, err = h.signing.Permit(t.Context(), w.ID, h.authorization(w, "CF", 0))
	// Then rejection precedes ledger and statistics writes.
	var failure *edgeclient.Error
	if !errors.As(err, &failure) || failure.Kind != edgeclient.Security {
		t.Fatalf("invalid final headers accepted: %v", err)
	}
	var count int
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM operations.request_attempts`).Scan(&count))
	require.Zero(t, count)
}
