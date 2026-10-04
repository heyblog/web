package announcement

import (
	"context"
	"errors"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mutationKind uint8

const (
	mutationEdit mutationKind = iota + 1
	mutationPublish
	mutationArchive
	mutationDelete
)

type mutation struct {
	kind    mutationKind
	id      string
	actor   string
	version int64
	input   Input
	publish PublishInput
	current dbgen.ContentAnnouncement
	actorID pgtype.UUID
}

func (repository *Repository) mutate(ctx context.Context, change mutation) (result ManagedAnnouncement, resultErr error) {
	identifier, err := parseID(change.id)
	if err != nil {
		return result, err
	}
	actorID, err := parseID(change.actor)
	if err != nil {
		return result, err
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return result, databaseError(err)
	}
	defer func() {
		if err := transaction.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			resultErr = errors.Join(resultErr, databaseError(err))
		}
	}()
	queries := repository.queries.WithTx(transaction)
	current, err := queries.LockAnnouncement(ctx, identifier)
	if err != nil {
		return result, databaseError(err)
	}
	if current.RowVersion != change.version {
		return result, conflict("announcement_version_conflict", "announcement was changed; reload before trying again")
	}
	change.current = current
	change.actorID = actorID
	row, err := repository.applyMutation(ctx, queries, change)
	if err != nil {
		return result, databaseError(err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return result, databaseError(err)
	}
	return mapRow(row, repository.now()), nil
}

func (repository *Repository) applyMutation(ctx context.Context, queries *dbgen.Queries, change mutation) (dbgen.ContentAnnouncement, error) {
	current, actor := change.current, change.actorID
	stateError := func() (dbgen.ContentAnnouncement, error) {
		return dbgen.ContentAnnouncement{}, conflict("announcement_state_conflict", "announcement lifecycle does not allow this operation")
	}
	switch change.kind {
	case mutationEdit:
		if current.Status == "ARCHIVED" {
			return stateError()
		}
		input := change.input
		if current.Status == "PUBLISHED" && !current.StartsAt.Time.After(repository.now()) && (input.Kind != current.Kind || input.StartsAt == nil || !input.StartsAt.Equal(current.StartsAt.Time)) {
			return stateError()
		}
		return queries.UpdateAnnouncement(ctx, dbgen.UpdateAnnouncementParams{ID: current.ID, ExpectedRowVersion: change.version, ActorID: actor, Kind: input.Kind, Title: input.Title, BodyMarkdown: input.BodyMarkdown, Priority: input.Priority, ActionType: input.ActionType, ActionLabel: input.ActionLabel, ActionPath: input.ActionPath, ActionExternalUrl: input.ActionExternalURL, StartsAt: databaseTime(input.StartsAt), EndsAt: databaseTime(input.EndsAt)})
	case mutationPublish:
		if current.Status != "DRAFT" {
			return stateError()
		}
		start := change.publish.StartsAt
		if start == nil {
			now := repository.now()
			start = &now
		}
		if change.publish.EndsAt != nil && !change.publish.EndsAt.After(*start) {
			return dbgen.ContentAnnouncement{}, validation("end time must be after start time")
		}
		return queries.PublishAnnouncement(ctx, dbgen.PublishAnnouncementParams{ID: current.ID, ExpectedRowVersion: change.version, ActorID: actor, StartsAt: databaseTime(start), EndsAt: databaseTime(change.publish.EndsAt)})
	case mutationArchive:
		if current.Status != "PUBLISHED" {
			return stateError()
		}
		return queries.ArchiveAnnouncement(ctx, dbgen.ArchiveAnnouncementParams{ID: current.ID, ExpectedRowVersion: change.version, ActorID: actor})
	case mutationDelete:
		if current.Status != "DRAFT" {
			return stateError()
		}
		_, err := queries.DeleteDraftAnnouncement(ctx, dbgen.DeleteDraftAnnouncementParams{ID: current.ID, ExpectedRowVersion: change.version})
		return current, err
	default:
		return stateError()
	}
}
