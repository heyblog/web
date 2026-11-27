package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"heyblog-api/internal/domain/site"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *Repository) SubmissionSite(ctx context.Context, shortID string) (Snapshot, error) {
	return repository.siteSnapshot(ctx, shortID, false)
}

func (repository *Repository) ResolveSite(ctx context.Context, shortID string) (Snapshot, error) {
	return repository.siteSnapshot(ctx, shortID, false)
}

func (repository *Repository) ResubmissionSite(ctx context.Context, shortID string) (Snapshot, error) {
	return repository.siteSnapshot(ctx, shortID, true)
}

func (repository *Repository) siteSnapshot(ctx context.Context, shortID string, resubmission bool) (Snapshot, error) {
	row, err := repository.queries.GetSiteByShortID(ctx, shortID)
	if err != nil {
		if isNotFound(err) {
			return Snapshot{}, newServiceError("site_not_found", http.StatusNotFound, "the target site was not found")
		}
		return Snapshot{}, fmt.Errorf("get target site by short ID: %w", err)
	}
	snapshot, err := loadSnapshot(ctx, repository.queries, row.ID)
	if isNotFound(err) || (err == nil && (snapshot.BlogStatus == site.BlogConfirmed) == resubmission) {
		return Snapshot{}, newServiceError("site_not_found", http.StatusNotFound, "the target site was not found")
	}
	return snapshot, err
}

func (repository *Repository) CreateSubmission(ctx context.Context, record submissionRecord) (string, error) {
	if record.Provenance.UserID != "" {
		return repository.createAccountSubmission(ctx, record)
	}
	if record.Action == ActionCreate {
		return repository.insertSubmission(ctx, record)
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin site submission: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bound := &Repository{pool: repository.pool, queries: repository.queries.WithTx(tx)}
	auditID, err := bound.insertSubmission(ctx, record)
	if err != nil {
		return "", err
	}
	return auditID, tx.Commit(ctx)
}

func (repository *Repository) insertSubmission(ctx context.Context, record submissionRecord) (string, error) {
	baseJSON, err := optionalSnapshotJSON(record.Action, record.Base)
	if err != nil {
		return "", err
	}
	proposedJSON, err := encodeSnapshot(record.Proposed)
	if err != nil {
		return "", err
	}
	siteID, err := parseOptionalUUID(record.Base.SiteID)
	if err != nil {
		return "", err
	}
	if siteID.Valid {
		row, lockErr := repository.queries.LockSiteByID(ctx, siteID)
		if lockErr != nil {
			return "", lockErr
		}
		if row.Revision != record.Base.Revision || site.BlogStatus(row.BlogStatus) != record.Base.BlogStatus {
			return "", newServiceError("site_revision_changed", http.StatusConflict, "the site changed; resubmit the application")
		}
	}
	userID, err := parseOptionalUUID(record.Provenance.UserID)
	if err != nil {
		return "", err
	}
	sourceSiteID, err := parseOptionalUUID(record.Provenance.SourceSiteID)
	if err != nil {
		return "", err
	}
	ownershipID, err := parseOptionalUUID(record.Provenance.OwnershipID)
	if err != nil {
		return "", err
	}
	channel := record.Provenance.Channel
	if channel == "" {
		channel = "ANONYMOUS"
	}
	row, err := repository.queries.CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{
		SubmitterUserID: userID, SourceChannel: channel, SourceSiteID: sourceSiteID, OwnershipID: ownershipID,
		LookupSecretHash: record.LookupHash, Action: string(record.Action), SiteID: siteID,
		BaseRevision: baseRevisionPointer(record.Action, record.Base), BaseSnapshot: baseJSON,
		ProposedSnapshot: proposedJSON, RequestReason: record.Input.Reason,
		SubmitterName: stringPointer(record.Input.Contact.Name), SubmitterEmail: stringPointer(record.Input.Contact.Email),
		NotifyByEmail: record.Input.Contact.NotifyByEmail,
	})
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && (databaseError.ConstraintName == "site_audits_pending_site_unique_idx" || databaseError.ConstraintName == "site_audits_pending_create_host_unique_idx") {
			return "", newServiceError("submission_pending", http.StatusConflict, "the site already has a pending submission")
		}
		return "", fmt.Errorf("create site audit: %w", err)
	}
	return uuidString(row.ID)
}

func (repository *Repository) LookupAudit(ctx context.Context, hash []byte) (Audit, error) {
	row, err := repository.queries.GetSiteAuditByLookupHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Audit{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit lookup credential was not found")
		}
		return Audit{}, fmt.Errorf("query site audit by lookup credential: %w", err)
	}
	return auditFromRow(row)
}
