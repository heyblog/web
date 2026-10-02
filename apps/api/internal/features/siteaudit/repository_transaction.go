package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/features/auth"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type auditTransaction struct {
	queries    *dbgen.Queries
	locked     dbgen.DirectorySiteAudit
	newShortID func() (string, error)
}

func (transaction *auditTransaction) LockAudit(ctx context.Context, auditID string) (Audit, error) {
	id, err := parseUUID(auditID)
	if err != nil {
		return Audit{}, err
	}
	row, err := transaction.queries.LockSiteAuditByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Audit{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit was not found")
		}
		return Audit{}, fmt.Errorf("lock site audit: %w", err)
	}
	transaction.locked = row
	// Decode snapshots only after the service checks status and draft revision.
	audit := Audit{ID: auditID, Action: Action(row.Action), Status: Status(row.Status), ReviewDraftRevision: row.ReviewDraftRevision}
	if row.SiteID.Valid {
		audit.SiteID, err = uuidString(row.SiteID)
	}
	return audit, err
}

func (transaction *auditTransaction) ReadLockedAudit() (Audit, error) {
	return auditFromRow(transaction.locked)
}

func (transaction *auditTransaction) LockSiteSnapshot(ctx context.Context, siteID string) (Snapshot, error) {
	id, err := parseUUID(siteID)
	if err != nil {
		return Snapshot{}, err
	}
	if _, err := transaction.queries.LockSiteByID(ctx, id); err != nil {
		return Snapshot{}, fmt.Errorf("lock target site: %w", err)
	}
	return loadSnapshot(ctx, transaction.queries, id)
}

func (transaction *auditTransaction) ResolveTaxonomy(ctx context.Context, reviewer auth.User, snapshot Snapshot) (Snapshot, error) {
	return resolveTaxonomy(ctx, transaction.queries, reviewer, snapshot)
}

func (transaction *auditTransaction) ApplySite(ctx context.Context, change reviewedSite, newShortID func() (string, error)) (Snapshot, error) {
	siteID, err := parseOptionalUUID(change.SiteID)
	if err != nil {
		return Snapshot{}, err
	}
	reviewerID, err := parseUUID(change.ReviewerID)
	if err != nil {
		return Snapshot{}, err
	}
	transaction.newShortID = newShortID
	id, revision, err := transaction.applySnapshot(ctx, transaction.queries, change.Action, change.Current, change.Final, siteID, reviewerID, change.CreatesProgramDependencies)
	if err != nil {
		return Snapshot{}, err
	}
	final := change.Final
	final.SiteID, _ = uuidString(id)
	final.Revision = revision
	return final, nil
}

func (transaction *auditTransaction) Approve(ctx context.Context, record decisionRecord) (Audit, error) {
	auditID, reviewerID, err := reviewRecordIDs(record.AuditID, record.ReviewerID)
	if err != nil {
		return Audit{}, err
	}
	siteID, err := parseUUID(record.Final.SiteID)
	if err != nil {
		return Audit{}, err
	}
	encoded, err := encodeSnapshot(record.Final)
	if err != nil {
		return Audit{}, err
	}
	updated, err := transaction.queries.ApproveSiteAudit(ctx, dbgen.ApproveSiteAuditParams{SiteID: siteID, FinalSnapshot: encoded, ReviewerComment: stringPointer(record.ReviewerComment), ReviewedBy: reviewerID, ID: auditID})
	if err != nil {
		return Audit{}, fmt.Errorf("approve site audit: %w", err)
	}
	return auditFromRow(updated)
}

func (transaction *auditTransaction) Reject(ctx context.Context, record decisionRecord) (Audit, error) {
	auditID, reviewerID, err := reviewRecordIDs(record.AuditID, record.ReviewerID)
	if err != nil {
		return Audit{}, err
	}
	updated, err := transaction.queries.RejectSiteAudit(ctx, dbgen.RejectSiteAuditParams{ReviewerComment: stringPointer(record.ReviewerComment), ReviewedBy: reviewerID, ID: auditID})
	if err != nil {
		return Audit{}, fmt.Errorf("reject site audit: %w", err)
	}
	return auditFromRow(updated)
}
