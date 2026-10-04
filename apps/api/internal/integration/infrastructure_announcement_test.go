//go:build integration

package integration_test

import (
	"context"
	"heyblog-api/internal/domain/content"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyAnnouncementConstraints(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	migrationURL string,
) {
	t.Helper()

	var actorID pgtype.UUID
	if err := connection.QueryRow(ctx, `
		INSERT INTO identity.users (email, username, display_name)
		VALUES ('announcement@example.com', 'announcement_admin', 'Announcement Admin')
		RETURNING id
	`).Scan(&actorID); err != nil {
		t.Fatalf("create announcement actor: %v", err)
	}

	now := time.Now().UTC()
	activeStart := now.Add(-2 * time.Hour)
	activeEnd := now.Add(2 * time.Hour)
	highPriorityID, highPriorityVersion := insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"MAIN",
		"High priority announcement",
		20,
		activeStart,
		&activeEnd,
	)
	_, _ = insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"MAIN",
		"Lower priority announcement",
		10,
		activeStart.Add(time.Minute),
		&activeEnd,
	)
	leading, err := dbgen.New(connection).GetLeadingActiveMainAnnouncement(ctx)
	if err != nil {
		t.Fatalf("get leading active main announcement: %v", err)
	}
	if leading.ID != highPriorityID || leading.Title != "High priority announcement" {
		t.Fatalf("leading active main announcement = %#v", leading)
	}

	rows, err := connection.Query(ctx, `
		SELECT title
		  FROM content.announcements
		 WHERE kind = 'MAIN'
		   AND status = 'PUBLISHED'
		   AND starts_at <= clock_timestamp()
		   AND (ends_at IS NULL OR ends_at > clock_timestamp())
		 ORDER BY priority DESC, starts_at DESC, id DESC
	`)
	if err != nil {
		t.Fatalf("query active main announcements: %v", err)
	}
	mainTitles, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect active main announcements: %v", err)
	}
	if !slices.Equal(mainTitles, []string{"High priority announcement", "Lower priority announcement"}) {
		t.Fatalf("active main announcement titles = %v", mainTitles)
	}

	bannerEnd := now.Add(time.Hour)
	_, _ = insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"BANNER",
		"Current banner",
		0,
		now.Add(-time.Hour),
		&bannerEnd,
	)
	if _, err := connection.Exec(ctx, `
		INSERT INTO content.announcements (
			kind, title, status, starts_at, ends_at, published_at,
			created_by, updated_by, published_by
		) VALUES ('BANNER', 'Overlapping banner', 'PUBLISHED', $1, $2, $3, $4, $4, $4)
	`, now, now.Add(30*time.Minute), now.Add(-time.Hour), actorID); err == nil {
		t.Fatal("overlapping published banner unexpectedly succeeded")
	}

	adjacentBannerID, adjacentBannerVersion := insertAnnouncement(
		ctx,
		t,
		connection,
		actorID,
		"BANNER",
		"Adjacent scheduled banner",
		0,
		bannerEnd,
		timePointer(bannerEnd.Add(time.Hour)),
	)
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements
		   SET title = 'Adjusted scheduled banner', updated_by = $2
		 WHERE id = $1 AND row_version = $3
	`, adjacentBannerID, actorID, adjacentBannerVersion); err != nil {
		t.Fatalf("update scheduled banner before public window: %v", err)
	}
	var scheduledRevisionCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*) FROM content.announcement_revisions WHERE announcement_id = $1
	`, adjacentBannerID).Scan(&scheduledRevisionCount); err != nil {
		t.Fatalf("count scheduled banner revisions: %v", err)
	}
	if scheduledRevisionCount != 0 {
		t.Fatalf("scheduled banner revision count = %d, want 0", scheduledRevisionCount)
	}
	if _, err := connection.Exec(ctx, `
		DELETE FROM content.announcements WHERE id = $1
	`, adjacentBannerID); err == nil {
		t.Fatal("scheduled announcement hard deletion unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO content.announcements (
			kind, title, status, action_type, action_label, action_external_url,
			created_by, updated_by
		) VALUES ('MAIN', 'Invalid internal action', 'DRAFT', 'INTERNAL', 'Read',
		          'https://example.com', $1, $1)
	`, actorID); err == nil {
		t.Fatal("internal action with external URL unexpectedly succeeded")
	}
	verifyAnnouncementRevisionActionConstraints(ctx, t, migrationURL, highPriorityID, actorID)

	var draftID pgtype.UUID
	if err := connection.QueryRow(ctx, `
		INSERT INTO content.announcements (
			kind, title, body_markdown, status, action_type, action_label,
			action_external_url, created_by, updated_by
		) VALUES ('MAIN', 'External action draft', 'Read [more](https://example.com)',
		          'DRAFT', 'EXTERNAL', 'Open', 'https://example.com', $1, $1)
		RETURNING id
	`, actorID).Scan(&draftID); err != nil {
		t.Fatalf("insert draft announcement with external action: %v", err)
	}
	deleteResult, err := connection.Exec(ctx, `DELETE FROM content.announcements WHERE id = $1`, draftID)
	if err != nil {
		t.Fatalf("delete draft announcement: %v", err)
	}
	if deleteResult.RowsAffected() != 1 {
		t.Fatalf("deleted draft rows = %d, want 1", deleteResult.RowsAffected())
	}

	var updatedVersion int64
	if err := connection.QueryRow(ctx, `
		UPDATE content.announcements
		   SET title = 'Corrected announcement',
		       body_markdown = 'Read **the correction**.',
		       action_type = 'INTERNAL',
		       action_label = 'Details',
		       action_path = '/announcements/correction',
		       updated_by = $2
		 WHERE id = $1 AND row_version = $3
		RETURNING row_version
	`, highPriorityID, actorID, highPriorityVersion).Scan(&updatedVersion); err != nil {
		t.Fatalf("update public announcement: %v", err)
	}
	if updatedVersion != highPriorityVersion+1 {
		t.Fatalf("updated row version = %d, want %d", updatedVersion, highPriorityVersion+1)
	}

	var revisionTitle string
	var revision int64
	if err := connection.QueryRow(ctx, `
		SELECT title, revision
		  FROM content.announcement_revisions
		 WHERE announcement_id = $1
	`, highPriorityID).Scan(&revisionTitle, &revision); err != nil {
		t.Fatalf("query public announcement revision: %v", err)
	}
	if revisionTitle != "High priority announcement" || revision != highPriorityVersion {
		t.Fatalf("public announcement revision = (%q, %d)", revisionTitle, revision)
	}

	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements SET status = 'DRAFT', updated_by = $2 WHERE id = $1
	`, highPriorityID, actorID); err == nil {
		t.Fatal("published announcement revert to draft unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements SET kind = 'BANNER', priority = 0, updated_by = $2 WHERE id = $1
	`, highPriorityID, actorID); err == nil {
		t.Fatal("public announcement kind change unexpectedly succeeded")
	}

	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements
		   SET status = 'ARCHIVED', archived_at = clock_timestamp(),
		       archived_by = $2, updated_by = $2
		 WHERE id = $1 AND row_version = $3
	`, highPriorityID, actorID, updatedVersion); err != nil {
		t.Fatalf("archive public announcement: %v", err)
	}
	var revisionCount int
	if err := connection.QueryRow(ctx, `
		SELECT count(*) FROM content.announcement_revisions WHERE announcement_id = $1
	`, highPriorityID).Scan(&revisionCount); err != nil {
		t.Fatalf("count archived announcement revisions: %v", err)
	}
	if revisionCount != 2 {
		t.Fatalf("archived announcement revision count = %d, want 2", revisionCount)
	}
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcements SET title = 'Tampered archive', updated_by = $2 WHERE id = $1
	`, highPriorityID, actorID); err == nil {
		t.Fatal("archived announcement update unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `DELETE FROM content.announcements WHERE id = $1`, highPriorityID); err == nil {
		t.Fatal("archived announcement hard deletion unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE content.announcement_revisions
		   SET title = 'Tampered revision'
		 WHERE announcement_id = $1
	`, highPriorityID); err == nil {
		t.Fatal("runtime revision update unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		DELETE FROM content.announcement_revisions WHERE announcement_id = $1
	`, highPriorityID); err == nil {
		t.Fatal("runtime revision deletion unexpectedly succeeded")
	}

	concurrentStart := now.Add(3 * time.Hour)
	concurrentEnd := now.Add(4 * time.Hour)
	results := make(chan error, 2)
	for _, title := range []string{"Concurrent banner A", "Concurrent banner B"} {
		go func(title string) {
			_, insertErr := connection.Exec(ctx, `
				INSERT INTO content.announcements (
					kind, title, status, starts_at, ends_at, published_at,
					created_by, updated_by, published_by
				) VALUES ('BANNER', $1, 'PUBLISHED', $2, $3, $4, $5, $5, $5)
			`, title, concurrentStart, concurrentEnd, now, actorID)
			results <- insertErr
		}(title)
	}
	concurrentSuccesses := 0
	for range 2 {
		if insertErr := <-results; insertErr == nil {
			concurrentSuccesses++
		}
	}
	if concurrentSuccesses != 1 {
		t.Fatalf("concurrent banner successes = %d, want 1", concurrentSuccesses)
	}
}

func verifyAnnouncementRevisionActionConstraints(
	ctx context.Context,
	t *testing.T,
	migrationURL string,
	announcementID pgtype.UUID,
	actorID pgtype.UUID,
) {
	t.Helper()
	connection, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		t.Fatalf("connect as migrator for announcement revision constraints: %v", err)
	}
	defer func() { _ = connection.Close(context.Background()) }()

	tests := []struct {
		name        string
		revision    int64
		actionType  string
		label       *string
		path        *string
		externalURL *string
	}{
		{
			name: "blank label", revision: 1001, actionType: "INTERNAL",
			label: stringPointer(" "), path: stringPointer("/valid"),
		},
		{
			name: "invalid internal path", revision: 1002, actionType: "INTERNAL",
			label: stringPointer("Open"), path: stringPointer("//example.com"),
		},
		{
			name: "invalid external URL", revision: 1003, actionType: "EXTERNAL",
			label: stringPointer("Open"), externalURL: stringPointer("ftp://example.com"),
		},
	}
	for _, testCase := range tests {
		if _, err := connection.Exec(ctx, `
			INSERT INTO content.announcement_revisions (
				announcement_id, revision, kind, title, priority,
				action_type, action_label, action_path, action_external_url,
				starts_at, published_at, published_by, changed_by
			) VALUES ($1, $2, 'MAIN', 'Invalid action revision', 0,
			          $3, $4, $5, $6, clock_timestamp(), clock_timestamp(), $7, $7)
		`, announcementID, testCase.revision, testCase.actionType, testCase.label, testCase.path, testCase.externalURL, actorID); err == nil {
			t.Errorf("%s revision unexpectedly succeeded", testCase.name)
		}
	}
}

func verifyAnnouncementQueries(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()
	queries := dbgen.New(connection)
	actor, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "announcement-query@example.com",
		Username:    "announcement_query_admin",
		DisplayName: "Announcement Query Admin",
	})
	if err != nil {
		t.Fatalf("create announcement query actor: %v", err)
	}

	body := "Read **the release notes**."
	draft, err := queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{
		Kind:         string(content.KindMain),
		Title:        "Release announcement",
		BodyMarkdown: &body,
		Priority:     50,
		ActionType:   string(content.ActionNone),
		ActorID:      actor.ID,
	})
	if err != nil {
		t.Fatalf("create announcement draft: %v", err)
	}
	if draft.Status != string(content.StatusDraft) || draft.RowVersion != 1 {
		t.Fatalf("created announcement = %#v", draft)
	}

	now := time.Now().UTC()
	published, err := queries.PublishAnnouncement(ctx, dbgen.PublishAnnouncementParams{
		ID:                 draft.ID,
		ExpectedRowVersion: draft.RowVersion,
		StartsAt:           pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		EndsAt:             pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("publish announcement: %v", err)
	}
	if published.Status != string(content.StatusPublished) || published.RowVersion != 2 {
		t.Fatalf("published announcement = %#v", published)
	}

	activeMain, err := queries.ListActiveMainAnnouncements(ctx)
	if err != nil {
		t.Fatalf("list active main announcements: %v", err)
	}
	if len(activeMain) != 1 || activeMain[0].ID != published.ID {
		t.Fatalf("active main announcements = %#v", activeMain)
	}

	correctedTitle := "Corrected release announcement"
	actionLabel := "Open release"
	actionPath := "/releases/current"
	updated, err := queries.UpdateAnnouncement(ctx, dbgen.UpdateAnnouncementParams{
		ID:                 published.ID,
		ExpectedRowVersion: published.RowVersion,
		Kind:               published.Kind,
		Title:              correctedTitle,
		BodyMarkdown:       published.BodyMarkdown,
		Priority:           published.Priority,
		ActionType:         string(content.ActionInternal),
		ActionLabel:        &actionLabel,
		ActionPath:         &actionPath,
		StartsAt:           published.StartsAt,
		EndsAt:             published.EndsAt,
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("update announcement through sqlc: %v", err)
	}
	if updated.Title != correctedTitle || updated.RowVersion != 3 {
		t.Fatalf("updated announcement = %#v", updated)
	}

	revisions, err := queries.ListAnnouncementRevisions(ctx, published.ID)
	if err != nil {
		t.Fatalf("list announcement revisions: %v", err)
	}
	if len(revisions) != 1 || revisions[0].Title != "Release announcement" || revisions[0].Revision != 2 {
		t.Fatalf("announcement revisions = %#v", revisions)
	}

	bannerDraft, err := queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{
		Kind:       string(content.KindBanner),
		Title:      "Current banner query",
		Priority:   0,
		ActionType: string(content.ActionNone),
		ActorID:    actor.ID,
	})
	if err != nil {
		t.Fatalf("create banner draft: %v", err)
	}
	banner, err := queries.PublishAnnouncement(ctx, dbgen.PublishAnnouncementParams{
		ID:                 bannerDraft.ID,
		ExpectedRowVersion: bannerDraft.RowVersion,
		StartsAt:           pgtype.Timestamptz{Time: now.Add(-30 * time.Minute), Valid: true},
		EndsAt:             pgtype.Timestamptz{Time: now.Add(30 * time.Minute), Valid: true},
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("publish banner: %v", err)
	}
	activeBanner, err := queries.GetActiveBannerAnnouncement(ctx)
	if err != nil {
		t.Fatalf("get active banner: %v", err)
	}
	if activeBanner.ID != banner.ID {
		t.Fatalf("active banner ID = %v, want %v", activeBanner.ID, banner.ID)
	}
	if _, err := queries.ArchiveAnnouncement(ctx, dbgen.ArchiveAnnouncementParams{
		ID:                 banner.ID,
		ExpectedRowVersion: banner.RowVersion,
		ActorID:            actor.ID,
	}); err != nil {
		t.Fatalf("archive banner: %v", err)
	}

	archived, err := queries.ArchiveAnnouncement(ctx, dbgen.ArchiveAnnouncementParams{
		ID:                 updated.ID,
		ExpectedRowVersion: updated.RowVersion,
		ActorID:            actor.ID,
	})
	if err != nil {
		t.Fatalf("archive main announcement: %v", err)
	}
	if archived.Status != string(content.StatusArchived) {
		t.Fatalf("archived announcement status = %q", archived.Status)
	}
	publicArchive, err := queries.ListPublicAnnouncementArchive(ctx, dbgen.ListPublicAnnouncementArchiveParams{
		PageSize:   20,
		PageOffset: 0,
	})
	if err != nil {
		t.Fatalf("list public announcement archive: %v", err)
	}
	if len(publicArchive) != 1 || publicArchive[0].ID != archived.ID {
		t.Fatalf("public announcement archive = %#v", publicArchive)
	}

	deletableDraft, err := queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{
		Kind:       string(content.KindMain),
		Title:      "Delete this draft",
		Priority:   0,
		ActionType: string(content.ActionNone),
		ActorID:    actor.ID,
	})
	if err != nil {
		t.Fatalf("create deletable announcement draft: %v", err)
	}
	deletedRows, err := queries.DeleteDraftAnnouncement(ctx, dbgen.DeleteDraftAnnouncementParams{ID: deletableDraft.ID, ExpectedRowVersion: deletableDraft.RowVersion})
	if err != nil {
		t.Fatalf("delete announcement draft through sqlc: %v", err)
	}
	if deletedRows != 1 {
		t.Fatalf("deleted announcement rows = %d, want 1", deletedRows)
	}
}

func insertAnnouncement(
	ctx context.Context,
	t *testing.T,
	connection *pgxpool.Pool,
	actorID pgtype.UUID,
	kind content.Kind,
	title string,
	priority int32,
	startsAt time.Time,
	endsAt *time.Time,
) (pgtype.UUID, int64) {
	t.Helper()
	var announcementID pgtype.UUID
	var rowVersion int64
	if err := connection.QueryRow(ctx, `
		INSERT INTO content.announcements (
			kind, title, status, priority, starts_at, ends_at, published_at,
			created_by, updated_by, published_by
		) VALUES ($1, $2, 'PUBLISHED', $3, $4, $5, $6, $7, $7, $7)
		RETURNING id, row_version
	`, kind, title, priority, startsAt, endsAt, time.Now().UTC(), actorID).Scan(
		&announcementID,
		&rowVersion,
	); err != nil {
		t.Fatalf("insert %s announcement %q: %v", kind, title, err)
	}
	return announcementID, rowVersion
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func stringPointer(value string) *string {
	return &value
}

func verifyAnnouncementActorDeletionSemantics(
	ctx context.Context,
	t *testing.T,
	runtimeConnection *pgxpool.Pool,
	migrationURL string,
) {
	t.Helper()
	queries := dbgen.New(runtimeConnection)
	actor, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "deleted-announcement-actor@example.com",
		Username:    "deleted_announcement_actor",
		DisplayName: "Deleted Announcement Actor",
	})
	if err != nil {
		t.Fatalf("create deletable announcement actor: %v", err)
	}

	now := time.Now().UTC()
	announcementID, initialVersion := insertAnnouncement(
		ctx,
		t,
		runtimeConnection,
		actor.ID,
		"MAIN",
		"Announcement with deletable actor",
		0,
		now.Add(-time.Hour),
		timePointer(now.Add(time.Hour)),
	)
	var updatedVersion int64
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE content.announcements
		   SET title = 'Announcement with deleted actor', updated_by = $2
		 WHERE id = $1 AND row_version = $3
		RETURNING row_version
	`, announcementID, actor.ID, initialVersion).Scan(&updatedVersion); err != nil {
		t.Fatalf("update announcement before actor deletion: %v", err)
	}
	var archivedVersion int64
	if err := runtimeConnection.QueryRow(ctx, `
		UPDATE content.announcements
		   SET status = 'ARCHIVED', archived_at = clock_timestamp(),
		       archived_by = $2, updated_by = $2
		 WHERE id = $1 AND row_version = $3
		RETURNING row_version
	`, announcementID, actor.ID, updatedVersion).Scan(&archivedVersion); err != nil {
		t.Fatalf("archive announcement before actor deletion: %v", err)
	}

	migrationConnection, err := pgx.Connect(ctx, migrationURL)
	if err != nil {
		t.Fatalf("connect as migrator for announcement actor deletion: %v", err)
	}
	defer func() { _ = migrationConnection.Close(context.Background()) }()
	if _, err := migrationConnection.Exec(ctx, "DELETE FROM identity.users WHERE id = $1", actor.ID); err != nil {
		t.Fatalf("hard delete announcement actor as migrator: %v", err)
	}

	var actorReferencesCleared bool
	var currentVersion int64
	if err := migrationConnection.QueryRow(ctx, `
		SELECT created_by IS NULL
		       AND updated_by IS NULL
		       AND published_by IS NULL
		       AND archived_by IS NULL,
		       row_version
		  FROM content.announcements
		 WHERE id = $1
	`, announcementID).Scan(&actorReferencesCleared, &currentVersion); err != nil {
		t.Fatalf("query announcement after actor deletion: %v", err)
	}
	if !actorReferencesCleared {
		t.Fatal("announcement retained actor references after actor deletion")
	}
	if currentVersion != archivedVersion {
		t.Fatalf("announcement row version after actor deletion = %d, want %d", currentVersion, archivedVersion)
	}

	var revisionCount int
	var revisionActorReferencesCleared bool
	if err := migrationConnection.QueryRow(ctx, `
		SELECT count(*), bool_and(published_by IS NULL AND changed_by IS NULL)
		  FROM content.announcement_revisions
		 WHERE announcement_id = $1
	`, announcementID).Scan(&revisionCount, &revisionActorReferencesCleared); err != nil {
		t.Fatalf("query announcement revisions after actor deletion: %v", err)
	}
	if revisionCount != 2 || !revisionActorReferencesCleared {
		t.Fatalf(
			"announcement revisions after actor deletion = count:%d actors-cleared:%t",
			revisionCount,
			revisionActorReferencesCleared,
		)
	}
}
