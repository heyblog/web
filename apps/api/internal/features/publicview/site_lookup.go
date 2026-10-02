package publicview

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

func (service *Service) SiteByIdentifier(
	ctx context.Context,
	identifier SiteIdentifier,
) (SiteProfile, error) {
	row, err := siteRowByIdentifier(ctx, service.lookup, identifier)
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return SiteProfile{}, err
	}
	return loadProfile(ctx, service.profile, profileLookup{row: row, err: err})
}

func siteRowByIdentifier(ctx context.Context, queries IdentifierQueries, identifier SiteIdentifier) (dbgen.DirectorySite, error) {
	var (
		row dbgen.DirectorySite
		err error
	)
	switch identifier.Kind {
	case IdentifierUUID:
		var id pgtype.UUID
		if scanErr := id.Scan(identifier.Value); scanErr != nil || !id.Valid {
			return dbgen.DirectorySite{}, badIdentifier("identifier")
		}
		row, err = queries.GetSiteByID(ctx, id)
	case IdentifierShortID:
		if validateErr := site.ValidateShortID(identifier.Value); validateErr != nil {
			return dbgen.DirectorySite{}, badIdentifier("identifier")
		}
		row, err = queries.GetSiteByShortID(ctx, identifier.Value)
	default:
		return dbgen.DirectorySite{}, badIdentifier("identifier")
	}
	return row, err
}

func (service *Service) SiteByCustomID(ctx context.Context, customID string) (SiteProfile, error) {
	if err := site.ValidateCustomID(customID); err != nil {
		return SiteProfile{}, badIdentifier("customId")
	}
	row, err := service.lookup.GetSiteByCustomID(ctx, &customID)
	return loadProfile(ctx, service.profile, profileLookup{row: row, err: err})
}
