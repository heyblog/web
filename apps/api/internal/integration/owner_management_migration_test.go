//go:build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"heyblog-api/internal/features/siteaudit"
)

func TestOwnerManagementUpgradePreservesMainDataAndLabels(t *testing.T) {
	// Given a deployed main database with pending audit history and an existing friend edge.
	f := newAuditMigrationFixture(t)
	ctx := t.Context()
	_, err := f.provider.UpTo(ctx, 22)
	require.NoError(t, err)
	legacy := f.pendingCreate(t, "pre-owner-audit.example.test")
	history := f.evidenceOutsideTaxonomy(t)
	source := insertSite(ctx, t, f.pool, "Upgrade01", "Existing source", "upgrade-source.example.test")
	insertSite(ctx, t, f.pool, "Upgrade02", "Existing target", "upgrade-target.example.test")
	_, err = f.pool.Exec(ctx, `SELECT directory.upsert_registered_friend_link($1,'https://upgrade-target.example.test/','upgrade-target.example.test','ACTIVE')`, source)
	require.NoError(t, err)

	// When the owner migrations apply, the old audit and friend edge survive.
	_, err = f.provider.Up(ctx)
	require.NoError(t, err)
	version, err := f.provider.GetDBVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(25), version)
	require.Equal(t, history, f.evidenceOutsideTaxonomy(t))
	assertSiteAuditStatus(t, ctx, f.pool, legacy.AuditID, siteaudit.StatusPending)
	var active bool
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM directory.list_friend_links($1,true) WHERE target_host='upgrade-target.example.test' AND link_status='ACTIVE')`, source).Scan(&active)
	require.NoError(t, err)
	require.True(t, active)

	// Then an owner can choose a multilingual label, approve it, and edit without losing it.
	owner := newOwnerWorkflowFixture(ctx, t, f.pool)
	input := owner.input
	input.Site.Tags = append([]siteaudit.TagInput(nil), input.Site.Tags...)
	input.Site.URL = "https://owner-source.example.test/"
	input.Site.Summary = "Owner summary using an alternate classification label"
	var labelID string
	err = f.pool.QueryRow(ctx, `INSERT INTO directory.tag_labels(tag_id,name,is_enabled) VALUES($1::uuid,'Owner alternate label',true) RETURNING id::text`, input.Site.Tags[0].ID).Scan(&labelID)
	require.NoError(t, err)
	input.Site.Tags[0].LabelID = labelID
	for _, summary := range []string{input.Site.Summary, "Owner summary updated while retaining its label"} {
		input.Site.Summary = summary
		before, err := owner.accounts.Site(ctx, owner.user, "Owner0001")
		require.NoError(t, err)
		request, err := owner.accounts.Submit(ctx, owner.user, "Owner0001", "OWNER_UPDATE", input)
		require.NoError(t, err)
		approved, err := owner.audits.Review(ctx, owner.reviewer, siteaudit.ReviewInput{AuditID: request.AuditID, Decision: siteaudit.DecisionApprove, ExpectedSiteRevision: before.Revision})
		require.NoError(t, err)
		after, err := owner.accounts.Site(ctx, owner.user, "Owner0001")
		require.NoError(t, err)
		require.Equal(t, after.Revision, approved.FinalSnapshot.Revision)
		require.Equal(t, labelID, after.Classification.Level1.LabelID)
		require.Equal(t, "Owner alternate label", after.Classification.Level1.Name)
		require.Equal(t, summary, after.Summary)
	}

	// Rolling back only the owner feature keeps main data and permits a clean re-upgrade.
	_, err = f.provider.DownTo(ctx, 22)
	require.NoError(t, err)
	var ownerTablesAbsent bool
	err = f.pool.QueryRow(ctx, `SELECT to_regclass('directory.site_ownerships') IS NULL AND to_regclass('directory.owner_friend_link_requests') IS NULL`).Scan(&ownerTablesAbsent)
	require.NoError(t, err)
	require.True(t, ownerTablesAbsent)
	assertSiteAuditStatus(t, ctx, f.pool, legacy.AuditID, siteaudit.StatusPending)
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM directory.list_friend_links($1,true) WHERE target_host='upgrade-target.example.test' AND link_status='ACTIVE')`, source).Scan(&active)
	require.NoError(t, err)
	require.True(t, active)
	_, err = f.provider.Up(ctx)
	require.NoError(t, err)
	verifyDatabaseCatalog(ctx, t, f.admin)
}
