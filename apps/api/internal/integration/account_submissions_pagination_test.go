//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/siteaudit"
)

func TestAccountSubmissionPagination(t *testing.T) {
	fixture := newCredentialFixture(t)
	repository := siteaudit.NewRepository(fixture.pool)
	audits := siteaudit.NewService(siteaudit.Dependencies{Repository: repository, Auth: fixture.auth})
	siteaudit.RegisterAccountRoutes(fixture.router.API, siteaudit.NewAccountService(repository, audits), credentialWebToken, fixture.redis)
	ids := make([]string, 0, 100)

	for _, total := range []int{0, 49, 50, 51, 100} {
		t.Run(fmt.Sprintf("%d submissions", total), func(t *testing.T) {
			for len(ids) < total {
				var id string
				err := fixture.pool.QueryRow(t.Context(), `INSERT INTO directory.site_audits
					(lookup_secret_hash, action, proposed_snapshot, request_reason, submitter_user_id, source_channel, created_at)
					VALUES (sha256(($1::text)::bytea), 'CREATE',
					jsonb_build_object('name', $1::text, 'scheme', 'https', 'normalized_host', $1::text || '.example.test'),
					'', $2, 'ACCOUNT_SUBMISSION', '2026-01-01'::timestamptz + $3 * interval '1 second')
					RETURNING id::text`, fmt.Sprintf("pagination-%d", len(ids)), fixture.actor, len(ids)).Scan(&id)
				require.NoError(t, err)
				ids = append(ids, id)
			}
			expected := slices.Clone(ids)
			slices.Reverse(expected)
			for page := 1; page <= 3; page++ {
				response := fixture.request(http.MethodGet, fmt.Sprintf("/api/web/account/site-submissions?page=%d", page), "")
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var body struct {
					Items []struct {
						ID string `json:"id"`
					} `json:"items"`
					HasMore *bool `json:"has_more"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.NotNil(t, body.Items, "empty pages must return an array")
				require.NotNil(t, body.HasMore, "the response must state whether another page exists")
				require.Equal(t, page*50 < total, *body.HasMore, "page %d", page)
				start, end := min((page-1)*50, total), min(page*50, total)
				actual := make([]string, 0, len(body.Items))
				for _, item := range body.Items {
					actual = append(actual, item.ID)
				}
				require.Equal(t, expected[start:end], actual, "page %d must have no missing or duplicate submissions", page)
			}
		})
	}

	t.Run("lookahead does not decode a record outside the page", func(t *testing.T) {
		_, err := fixture.pool.Exec(t.Context(), `INSERT INTO directory.site_audits
			(lookup_secret_hash, action, proposed_snapshot, request_reason, submitter_user_id, source_channel, created_at)
			SELECT sha256(('lookahead-visible-' || n)::bytea), 'CREATE',
			jsonb_build_object('name', 'visible-' || n, 'normalized_host', 'visible-' || n || '.example.test'),
			'', $1, 'ACCOUNT_SUBMISSION', '2026-02-01'::timestamptz + n * interval '1 second'
			FROM generate_series(1, 50) n`, fixture.actor)
		require.NoError(t, err)
		_, err = fixture.pool.Exec(t.Context(), `INSERT INTO directory.site_audits
			(lookup_secret_hash, action, proposed_snapshot, request_reason, submitter_user_id, source_channel, created_at)
			VALUES (sha256('pagination-lookahead'::bytea), 'CREATE',
			'{"name": 42, "normalized_host": "pagination-lookahead.example.test"}',
			'', $1, 'ACCOUNT_SUBMISSION', '2026-02-01'::timestamptz)`, fixture.actor)
		require.NoError(t, err)
		response := fixture.request(http.MethodGet, "/api/web/account/site-submissions", "")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body siteaudit.AccountSubmissionPage
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		require.Len(t, body.Items, 50)
		require.True(t, body.HasMore)
	})
}
