package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (repository *Repository) SubmissionSite(ctx context.Context, shortID string) (Snapshot, error) {
	return repository.siteSnapshot(ctx, shortID, "get target site by short ID")
}

func (repository *Repository) ResolveSite(ctx context.Context, shortID string) (Snapshot, error) {
	return repository.siteSnapshot(ctx, shortID, "get site by short ID")
}

func (repository *Repository) siteSnapshot(ctx context.Context, shortID, operation string) (Snapshot, error) {
	row, err := repository.queries.GetSiteByShortID(ctx, shortID)
	if err != nil {
		if isNotFound(err) {
			return Snapshot{}, newServiceError("site_not_found", http.StatusNotFound, "the target site was not found")
		}
		return Snapshot{}, fmt.Errorf("%s: %w", operation, err)
	}
	snapshot, err := loadSnapshot(ctx, repository.queries, row.ID)
	if isNotFound(err) {
		return Snapshot{}, newServiceError("site_not_found", http.StatusNotFound, "the target site was not found")
	}
	return snapshot, err
}

func (repository *Repository) CreateSubmission(ctx context.Context, record submissionRecord) (string, error) {
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
	row, err := repository.queries.CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{
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
