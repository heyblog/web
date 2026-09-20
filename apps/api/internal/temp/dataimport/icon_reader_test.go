package dataimport

import (
	"context"

	"heyblog-api/internal/application/publicview"
)

func (importTestPublicViews) SiteIconByIdentifier(context.Context, publicview.SiteIdentifier) (publicview.SiteIcon, error) {
	return publicview.SiteIcon{}, nil
}
