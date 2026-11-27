package siteaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type annotationFilter struct {
	Status string
	Query  string
	Page   int32
}

func (repo *Repository) annotationSites(ctx context.Context, filter annotationFilter) (annotationPage[AnnotationSite], error) {
	page := annotationPage[AnnotationSite]{Page: filter.Page, PageSize: annotationPageSize}
	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return page, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repo.queries.WithTx(tx)
	rows, err := queries.ListAnnotationSites(ctx, dbgen.ListAnnotationSitesParams{
		BlogStatus: filter.Status, QueryText: filter.Query, PageSize: page.PageSize, PageOffset: (page.Page - 1) * page.PageSize,
	})
	if err != nil {
		return page, err
	}
	page.Total, err = queries.CountAnnotationSites(ctx, dbgen.CountAnnotationSitesParams{BlogStatus: filter.Status, QueryText: filter.Query})
	if err != nil {
		return page, err
	}
	page.Items = make([]AnnotationSite, 0, len(rows))
	for _, row := range rows {
		page.Items = append(page.Items, annotationSite(row.DirectorySite, row.PendingAuditID))
	}
	return page, tx.Commit(ctx)
}

func annotationSite(row dbgen.DirectorySite, pending string) AnnotationSite {
	return AnnotationSite{
		ShortID: row.ShortID, Name: row.Name, URL: annotationURL(row),
		BlogStatus: site.BlogStatus(row.BlogStatus), Revision: row.Revision, UpdatedAt: row.UpdatedAt.Time, PendingAuditID: pending,
	}
}

func annotationURL(row dbgen.DirectorySite) string {
	return row.Scheme + "://" + row.NormalizedHost + row.BasePath
}

func annotationNotFound() error {
	return newServiceError("site_not_found", http.StatusNotFound, "the site was not found")
}

func (repo *Repository) annotationDetail(ctx context.Context, shortID string) (AnnotationDetail, error) {
	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return AnnotationDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	row, err := q.GetSiteByShortID(ctx, shortID)
	if isNotFound(err) {
		return AnnotationDetail{}, annotationNotFound()
	}
	if err != nil {
		return AnnotationDetail{}, err
	}
	detail, err := loadAnnotation(ctx, q, row)
	if err != nil {
		return AnnotationDetail{}, err
	}
	return detail, tx.Commit(ctx)
}

func (repo *Repository) annotationHistory(ctx context.Context, shortID string, pageNumber int32) (annotationPage[AnnotationHistory], error) {
	page := annotationPage[AnnotationHistory]{Page: pageNumber, PageSize: annotationPageSize}
	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return page, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	row, err := q.GetSiteByShortID(ctx, shortID)
	if isNotFound(err) {
		return page, annotationNotFound()
	}
	if err != nil {
		return page, err
	}
	entries, err := q.ListSiteAnnotationHistory(ctx, dbgen.ListSiteAnnotationHistoryParams{
		SiteID: row.ID, PageSize: page.PageSize, PageOffset: (page.Page - 1) * page.PageSize,
	})
	if err != nil {
		return page, err
	}
	page.Total, err = q.CountSiteAnnotationHistory(ctx, row.ID)
	if err != nil {
		return page, err
	}
	page.Items = make([]AnnotationHistory, 0, len(entries))
	for _, entry := range entries {
		id, idErr := uuidString(entry.ID)
		if idErr != nil {
			return page, idErr
		}
		var auditID string
		if entry.SiteAuditID.Valid {
			auditID, idErr = uuidString(entry.SiteAuditID)
			if idErr != nil {
				return page, idErr
			}
		}
		page.Items = append(page.Items, AnnotationHistory{
			ID: id, ActorName: entry.ActorName, CreatedAt: entry.CreatedAt.Time, Revision: entry.SiteRevision,
			AuditID: auditID, Reason: stringValue(entry.Reason), Source: entry.Source,
			Before: entry.BeforeSnapshot, After: entry.AfterSnapshot,
		})
	}
	return page, tx.Commit(ctx)
}

func loadAnnotation(ctx context.Context, q *dbgen.Queries, row dbgen.DirectorySite) (AnnotationDetail, error) {
	feeds, err := q.ListSiteFeeds(ctx, row.ID)
	if err != nil {
		return AnnotationDetail{}, err
	}
	snapshot, err := loadSnapshotFromRows(ctx, q, row, feeds)
	if err != nil {
		return AnnotationDetail{}, err
	}
	pending, err := q.AnnotationPendingAudit(ctx, dbgen.AnnotationPendingAuditParams{SiteID: row.ID, Host: row.NormalizedHost})
	if err != nil {
		return AnnotationDetail{}, err
	}
	outgoing, err := q.ListRecordedOutgoingLinks(ctx, row.ID)
	if err != nil {
		return AnnotationDetail{}, err
	}
	incoming, err := q.ListRecordedIncomingLinks(ctx, row.NormalizedHost)
	if err != nil {
		return AnnotationDetail{}, err
	}
	return mapAnnotation(row, snapshot, feeds, pending, outgoing, incoming)
}

func mapAnnotation(
	row dbgen.DirectorySite, snapshot Snapshot, feeds []dbgen.DirectorySiteFeed, pending string,
	outgoing []dbgen.ListRecordedOutgoingLinksRow, incoming []dbgen.ListRecordedIncomingLinksRow,
) (AnnotationDetail, error) {
	detail := AnnotationDetail{
		AnnotationSite: annotationSite(row, pending), Summary: row.Summary, AccessScope: row.AccessScope, Visibility: row.Visibility,
		TagCascadeID: snapshot.TagCascadeID, Resources: snapshot.Resources,
		Tags: make([]AnnotationTag, 0, len(snapshot.Tags)), Components: make([]AnnotationComponent, 0, len(snapshot.Components)),
		Feeds: make([]AnnotationFeed, 0, len(snapshot.Feeds)), Outgoing: make([]AnnotationOutgoing, 0, len(outgoing)),
		Incoming: make([]AnnotationIncoming, 0, len(incoming)),
	}
	for _, tag := range snapshot.Tags {
		detail.Tags = append(detail.Tags, AnnotationTag{AnnotationSelection: AnnotationSelection{ID: tag.ID, Role: tag.Role}, Name: tag.Name, Level: tag.Level})
	}
	for _, component := range snapshot.Components {
		detail.Components = append(detail.Components, AnnotationComponent{AnnotationSelection: AnnotationSelection{ID: component.ID, Role: component.Role}, Name: component.Name})
	}
	for index, feed := range snapshot.Feeds {
		detail.Feeds = append(detail.Feeds, AnnotationFeed{
			FeedInput: FeedInput{Name: feed.Name, URL: feed.URL, Format: feed.Format, IsDefault: feed.IsDefault},
			ID:        feed.ID, IsEnabled: feeds[index].IsEnabled,
		})
	}
	for _, link := range outgoing {
		detail.Outgoing = append(detail.Outgoing, AnnotationOutgoing{
			AnnotationFriendInput: AnnotationFriendInput{TargetURL: link.TargetUrl, LinkStatus: link.LinkStatus}, TargetHost: link.TargetHost,
		})
	}
	for _, link := range incoming {
		detail.Incoming = append(detail.Incoming, AnnotationIncoming{
			SourceShortID: link.SourceShortID, SourceName: link.SourceName, SourceURL: link.SourceUrl, LinkStatus: link.LinkStatus,
		})
	}
	encoded, err := json.Marshal(outgoing)
	if err != nil {
		return AnnotationDetail{}, err
	}
	version := sha256.Sum256(encoded)
	detail.FriendVersion = hex.EncodeToString(version[:])
	return detail, nil
}
