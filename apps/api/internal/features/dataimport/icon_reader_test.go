package dataimport

import (
	"context"

	"heyblog-api/internal/features/publicview"
)

func (importTestPublicViews) SiteIconByIdentifier(context.Context, publicview.SiteIdentifier) (publicview.SiteIcon, error) {
	return publicview.SiteIcon{}, nil
}
