package announcement

import (
	"context"
	"errors"
	"strconv"
	"time"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
	now     func() time.Time
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: dbgen.New(pool), now: time.Now}
}

func parseID(id string) (pgtype.UUID, error) {
	var value pgtype.UUID
	if err := value.Scan(id); err != nil || !value.Valid {
		return value, validation("announcement identifier is invalid")
	}
	return value, nil
}
func databaseError(err error) error {
	if err == nil {
		return nil
	}
	var known *apperror.Error
	if errors.As(err, &known) {
		return known
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.New(apperror.KindNotFound, "not_found", "announcement was not found")
	}
	var failure *pgconn.PgError
	if errors.As(err, &failure) {
		switch failure.Code {
		case "23P01":
			return conflict("banner_window_conflict", "another banner overlaps this publication window")
		case "23514":
			switch failure.ConstraintName {
			case "announcements_status_transition_check", "announcements_public_identity_check", "announcements_delete_draft_only_check":
				return conflict("announcement_state_conflict", "announcement lifecycle does not allow this operation")
			default:
				return validation("announcement fields violate publication rules")
			}
		}
	}
	return apperror.Wrap(err, apperror.KindInternal, "internal_error", "announcement service is unavailable", "announcement database operation")
}

func (repository *Repository) Get(ctx context.Context, id string) (ManagedAnnouncement, error) {
	identifier, err := parseID(id)
	if err != nil {
		return ManagedAnnouncement{}, err
	}
	row, err := repository.queries.GetAnnouncementByID(ctx, identifier)
	if err != nil {
		return ManagedAnnouncement{}, databaseError(err)
	}
	return mapRow(row, repository.now()), nil
}

func (repository *Repository) Create(ctx context.Context, actor string, input Input) (ManagedAnnouncement, error) {
	actorID, err := parseID(actor)
	if err != nil {
		return ManagedAnnouncement{}, err
	}
	row, err := repository.queries.CreateAnnouncement(ctx, dbgen.CreateAnnouncementParams{Kind: input.Kind, Title: input.Title, BodyMarkdown: input.BodyMarkdown, Priority: input.Priority, ActionType: input.ActionType, ActionLabel: input.ActionLabel, ActionPath: input.ActionPath, ActionExternalUrl: input.ActionExternalURL, StartsAt: databaseTime(input.StartsAt), EndsAt: databaseTime(input.EndsAt), ActorID: actorID})
	if err != nil {
		return ManagedAnnouncement{}, databaseError(err)
	}
	return mapRow(row, repository.now()), nil
}

func (repository *Repository) List(ctx context.Context, query ListQuery) (List, error) {
	var kind, status *string
	if query.Kind != "" {
		kind = &query.Kind
	}
	if query.Status != "" {
		status = &query.Status
	}
	total, err := repository.queries.CountAnnouncementsForManagement(ctx, dbgen.CountAnnouncementsForManagementParams{FilterKind: kind, FilterStatus: status})
	if err != nil {
		return List{}, databaseError(err)
	}
	rows, err := repository.queries.ListAnnouncementsForManagement(ctx, dbgen.ListAnnouncementsForManagementParams{FilterKind: kind, FilterStatus: status, PageSize: query.PageSize, PageOffset: (query.Page - 1) * query.PageSize})
	if err != nil {
		return List{}, databaseError(err)
	}
	result := List{Announcements: []ManagedAnnouncement{}, Total: total, Page: query.Page, PageSize: query.PageSize}
	for _, row := range rows {
		result.Announcements = append(result.Announcements, mapRow(dbgen.ContentAnnouncement{ID: row.ID, Kind: row.Kind, Title: row.Title, BodyMarkdown: row.BodyMarkdown, Status: row.Status, Priority: row.Priority, ActionType: row.ActionType, ActionLabel: row.ActionLabel, ActionPath: row.ActionPath, ActionExternalUrl: row.ActionExternalUrl, StartsAt: row.StartsAt, EndsAt: row.EndsAt, PublishedAt: row.PublishedAt, ArchivedAt: row.ArchivedAt, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, PublishedBy: row.PublishedBy, ArchivedBy: row.ArchivedBy, RowVersion: row.RowVersion, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, repository.now()))
	}
	return result, nil
}

func (repository *Repository) Revisions(ctx context.Context, id string) ([]Revision, error) {
	identifier, err := parseID(id)
	if err != nil {
		return nil, err
	}
	if _, err := repository.queries.GetAnnouncementByID(ctx, identifier); err != nil {
		return nil, databaseError(err)
	}
	rows, err := repository.queries.ListAnnouncementRevisions(ctx, identifier)
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]Revision, 0, len(rows))
	for _, row := range rows {
		result = append(result, Revision{Input: Input{Kind: row.Kind, Title: row.Title, BodyMarkdown: row.BodyMarkdown, Priority: row.Priority, ActionType: row.ActionType, ActionLabel: row.ActionLabel, ActionPath: row.ActionPath, ActionExternalURL: row.ActionExternalUrl, StartsAt: nullableTime(row.StartsAt), EndsAt: nullableTime(row.EndsAt)}, Revision: strconv.FormatInt(row.Revision, 10), PublishedAt: row.PublishedAt.Time, PublishedBy: nullableID(row.PublishedBy), ChangedAt: row.ChangedAt.Time, ChangedBy: nullableID(row.ChangedBy)})
	}
	return result, nil
}
