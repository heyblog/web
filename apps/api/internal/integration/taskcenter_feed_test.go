//go:build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"

	"heyblog-api/internal/features/taskcenter"

	"heyblog.local/taskprotocol"

	"github.com/stretchr/testify/require"
)

var validTaskFeed = []byte(`<rss><channel><item><guid>one</guid><title>Article</title><link>https://feed.example.test/article</link></item></channel></rss>`)

func newFeedTask(t *testing.T) (*taskHarness, *taskprotocol.Work, string) {
	t.Helper()
	h := newTaskHarness(t, "rss")
	siteID := insertSite(t.Context(), t, h.fixture.pool, "FeedTest1", "Feed", "feed.example.test")
	var feedID string
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `INSERT INTO directory.site_feeds(site_id,name,location_type,url_ref,url_key,format,is_enabled,is_default)
		VALUES($1,'RSS','RELATIVE','/feed.xml','/feed.xml','RSS',true,true) RETURNING id::text`, siteID).Scan(&feedID))
	return h, h.start(t), feedID
}

func TestRSSPersistence_when_ValidatorCannotBeStored(t *testing.T) {
	// Given an opaque ETag that cannot be stored as PostgreSQL text.
	h, w, feedID := newFeedTask(t)
	// When the valid feed is committed.
	e := h.report(t, w, w.Route[0], 0, 200, validTaskFeed, http.Header{"Etag": {"\"\xe9\""}, "Last-Modified": {"invalid date"}})
	require.True(t, e.Valid)
	h.finish(t, w)
	// Then content survives while unusable validators are omitted.
	var etag, modified, state string
	var count int
	require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT etag,last_modified,(SELECT count(*) FROM operations.feed_items),
		(SELECT state FROM operations.targets WHERE id=$2) FROM operations.feed_documents WHERE feed_id=$1`, feedID, w.ID).Scan(&etag, &modified, &count, &state))
	require.Empty(t, etag)
	require.Empty(t, modified)
	require.Equal(t, 1, count)
	require.Equal(t, "succeeded", state)
}

func TestRSSRequest_when_StoredValidatorsAreMalformed(t *testing.T) {
	// Given legacy validators that would poison an outbound header.
	h, w, feedID := newFeedTask(t)
	_, err := h.fixture.pool.Exec(t.Context(), `INSERT INTO operations.feed_documents(feed_id,etag,last_modified) VALUES($1,$2,$3)`, feedID, "bad\r\nvalue", "invalid date")
	require.NoError(t, err)
	// When obtaining the actual outbound authorization.
	permit, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, w.Route[0], 0))
	require.NoError(t, err)
	// Then unsafe values are absent from the request.
	require.Empty(t, permit.Headers["If-None-Match"])
	require.Empty(t, permit.Headers["If-Modified-Since"])
}

func TestRSSNotModified_when_ValidatorsAreRefreshed(t *testing.T) {
	for _, scenario := range []struct{ name, etag, modified, wantETag, wantModified string }{
		{"new etag", `W/"new"`, "", `W/"new"`, "Wed, 21 Oct 2015 07:28:00 GMT"},
		{"new date", "", "Thu, 22 Oct 2015 07:28:00 GMT", `"old"`, "Thu, 22 Oct 2015 07:28:00 GMT"},
		{"unsupported etag", "\"\xe9\"", "", `"old"`, "Wed, 21 Oct 2015 07:28:00 GMT"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given a cached document and a conditional response.
			h, w, feedID := newFeedTask(t)
			_, err := h.fixture.pool.Exec(t.Context(), `INSERT INTO operations.feed_documents(feed_id,etag,last_modified,checked_at)
				VALUES($1,'"old"','Wed, 21 Oct 2015 07:28:00 GMT','2000-01-01')`, feedID)
			require.NoError(t, err)
			// When the response refreshes validators.
			e := h.report(t, w, w.Route[0], 0, 304, nil, http.Header{"Etag": {scenario.etag}, "Last-Modified": {scenario.modified}})
			// Then no response body or fallback is needed.
			require.True(t, e.Valid)
			var etag, modified string
			var checked time.Time
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT etag,last_modified,checked_at FROM operations.feed_documents WHERE feed_id=$1`, feedID).Scan(&etag, &modified, &checked))
			require.Equal(t, scenario.wantETag, etag)
			require.Equal(t, scenario.wantModified, modified)
			require.True(t, checked.After(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)))
		})
	}
}

func TestRSSResultRejectsChangedIdentity(t *testing.T) {
	for _, change := range []string{
		`UPDATE directory.sites SET monitoring_generation=monitoring_generation+1`,
		`UPDATE directory.site_feeds SET is_enabled=false,is_default=false`,
		`UPDATE directory.site_feeds SET url_ref='/changed.xml',url_key='/changed.xml'`,
	} {
		t.Run(change, func(t *testing.T) {
			// Given a response in flight for the original source.
			h, w, feedID := newFeedTask(t)
			p, err := h.signing.Permit(t.Context(), w.ID, h.authorization(w, w.Route[0], 0))
			require.NoError(t, err)
			require.NoError(t, h.repo.Sent(t.Context(), w.ID, taskcenter.Send{Fence: h.fence(w), AttemptID: p.ID}))
			_, err = h.fixture.pool.Exec(t.Context(), change)
			require.NoError(t, err)
			// When that response arrives after identity mutation.
			_, err = h.execution.Report(t.Context(), w.ID, taskprotocol.Report{Fence: h.fence(w), AttemptID: p.ID, Status: 200, Body: validTaskFeed})
			// Then it cannot write content or validators.
			require.Error(t, err)
			var count int
			require.NoError(t, h.fixture.pool.QueryRow(t.Context(), `SELECT count(*) FROM operations.feed_documents WHERE feed_id=$1`, feedID).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestRSSPersistence_when_JSONContainsNUL(t *testing.T) {
	// Given a syntactically valid but unstorable feed.
	h, w, _ := newFeedTask(t)
	body := []byte(`{"version":"https://jsonfeed.org/version/1.1","items":[{"url":"https://feed.example.test/article","title":"\u0000"}]}`)
	// When both persisted routes return it.
	for _, platform := range w.Route {
		e := h.report(t, w, platform, 0, 200, body, nil)
		require.False(t, e.Valid)
		require.True(t, e.Progress.Done)
		require.Equal(t, "invalid_feed", e.Reason)
	}
	result := h.finish(t, w)
	// Then parsing failure is terminal and content remains empty.
	require.False(t, result.BusinessValid)
	require.True(t, result.Fallback)
}

func TestRSSNotModifiedRequiresCachedDocument(t *testing.T) {
	// Given a new feed without a cached valid document.
	h, w, _ := newFeedTask(t)
	// When a platform incorrectly returns 304.
	e := h.report(t, w, w.Route[0], 0, 304, nil, nil)
	// Then it does not count as a successful fetch.
	require.False(t, e.Valid)
	require.Equal(t, "invalid_feed", e.Reason)
}
