package publicview

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/siteicon"
)

type SiteIcon struct {
	Content []byte
	Hash    string
}

func (service *Service) SiteIconByIdentifier(ctx context.Context, identifier SiteIdentifier) (SiteIcon, error) {
	row, err := service.siteRowByIdentifier(ctx, identifier)
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return SiteIcon{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) || err == nil && row.Visibility == "REMOVED" {
		return SiteIcon{}, notFound()
	}
	if err != nil {
		return SiteIcon{}, iconUnavailable(err)
	}
	icon, err := service.queries.GetSiteIcon(ctx, row.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteIcon{}, notFound()
	}
	if err != nil {
		return SiteIcon{}, iconUnavailable(err)
	}
	content, err := siteicon.Normalize(icon.Content)
	if err != nil {
		return SiteIcon{}, notFound()
	}
	return SiteIcon{Content: content, Hash: hex.EncodeToString(icon.Sha256)}, nil
}

func iconUnavailable(err error) error {
	return apperror.Wrap(err, apperror.KindUnavailable, apperror.CodeServiceUnavailable,
		"site icon is temporarily unavailable", "load cached site icon")
}
