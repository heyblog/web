package siteaudit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type blogClassificationChange struct {
	SiteID     pgtype.UUID
	ReviewerID pgtype.UUID
	AuditID    pgtype.UUID
	Revision   int64
	Previous   site.BlogStatus
}

func recordAuditBlogClassification(ctx context.Context, queries *dbgen.Queries, change blogClassificationChange) error {
	before, err := json.Marshal(struct {
		BlogStatus site.BlogStatus `json:"blog_status,omitempty"`
	}{BlogStatus: change.Previous})
	if err != nil {
		return err
	}
	if err := queries.InsertSiteAnnotationHistory(ctx, dbgen.InsertSiteAnnotationHistoryParams{
		SiteID: change.SiteID, ActorID: change.ReviewerID, SiteRevision: change.Revision,
		Source: "AUDIT", SiteAuditID: change.AuditID, BeforeSnapshot: before,
		AfterSnapshot: []byte(`{"blog_status":"BLOG"}`),
	}); err != nil {
		return fmt.Errorf("record reviewed blog classification: %w", err)
	}
	return nil
}
