package publicview

import (
	"context"
	"errors"
	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
)

type directoryLabelQueries interface {
	ResolveDirectoryLabel(context.Context, dbgen.ResolveDirectoryLabelParams) (dbgen.ResolveDirectoryLabelRow, error)
}

func resolveDirectoryLabels(ctx context.Context, q directoryLabelQueries, query DirectoryQuery) (DirectoryQuery, error) {
	resolve := func(slug string, labels []string) (string, string, error) {
		if slug == "" {
			return "", "", nil
		}
		row, err := q.ResolveDirectoryLabel(ctx, dbgen.ResolveDirectoryLabelParams{Slug: slug, LabelIds: labels})
		if errors.Is(err, pgx.ErrNoRows) {
			return slug, "", nil
		}
		if err != nil {
			return "", "", internalError(err, "resolve directory display name")
		}
		return row.Slug, uuidText(row.LabelID), nil
	}
	var err error
	query.Level1, query.Level1LabelID, err = resolve(query.Level1, []string{query.Level1LabelID})
	if err != nil {
		return query, err
	}
	query.Level2, query.Level2LabelID, err = resolve(query.Level2, []string{query.Level2LabelID})
	if err != nil {
		return query, err
	}
	slugs := []string{}
	labels := []string{}
	seen := map[string]bool{}
	for _, slug := range query.TertiaryTags {
		canonical, selected, err := resolve(slug, query.TertiaryLabelIDs)
		if err != nil {
			return query, err
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		slugs = append(slugs, canonical)
		labels = append(labels, selected)
	}
	query.TertiaryTags = slugs
	query.TertiaryLabelIDs = labels
	return query, nil
}
