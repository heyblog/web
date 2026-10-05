package dataimport

import (
	"context"
	"heyblog-api/internal/features/publicview"
)

func (importTestPublicViews) Graph(context.Context) (publicview.FriendGraph, error) {
	return publicview.FriendGraph{}, nil
}

func (importTestPublicViews) SiteGraphByIdentifier(context.Context, publicview.SiteIdentifier) (publicview.FriendGraph, error) {
	return publicview.FriendGraph{}, nil
}
