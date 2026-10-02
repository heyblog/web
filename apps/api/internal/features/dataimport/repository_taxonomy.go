package dataimport

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// ImportTaxonomy atomically replaces only imported taxonomy and assignments.
func (repository *Repository) ImportTaxonomy(ctx context.Context, bundle TagTaxonomyBundle) (Counts, error) {
	if repository.pool == nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("database pool is unavailable"))
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, tag := range bundle.Tags {
		if tag.Source == "SQLITE" {
			result, updateErr := tx.Exec(ctx, `UPDATE directory.tags SET name=$1, description=COALESCE($2,''), is_fixed=true WHERE system_key=$3`, tag.Name, tag.Description, tag.TagID)
			if updateErr != nil || result.RowsAffected() != 1 {
				return Counts{}, fmt.Errorf("update fixed taxonomy tag %q: %w", tag.TagID, updateErr)
			}
			continue
		}
		id, parseErr := parseUUIDText(tag.TagID)
		if parseErr != nil {
			return Counts{}, fmt.Errorf("legacy taxonomy tag %q: %w", tag.TagID, parseErr)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO directory.tags (id,name,normalized_name,slug,description,is_enabled) VALUES ($1,$2,lower(btrim($2)),$3,COALESCE($4,''),true) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description`, id, tag.Name, "legacy-"+tag.TagID, tag.Description); err != nil {
			return Counts{}, fmt.Errorf("insert legacy taxonomy tag: %w", err)
		}
	}
	for _, site := range bundle.Sites {
		result, updateErr := tx.Exec(ctx, `UPDATE directory.sites SET tag_cascade_id=(SELECT id FROM directory.tag_cascades WHERE scope='SITE' AND taxonomy_key=$1) WHERE id=$2::uuid`, site.CascadeKey, site.SiteID)
		if updateErr != nil || result.RowsAffected() != 1 {
			return Counts{}, fmt.Errorf("update site cascade for %q: %w", site.SiteID, updateErr)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM directory.site_tags WHERE site_id=$1::uuid AND role='TERTIARY'`, site.SiteID); err != nil {
			return Counts{}, fmt.Errorf("clear site tertiary tags: %w", err)
		}
		for _, tag := range site.TertiaryTags {
			var result pgconn.CommandTag
			if tag.Source == "SQLITE" {
				result, err = tx.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role,assignment_source,position) SELECT $1::uuid,t.id,'TERTIARY','IMPORTED',$2 FROM directory.tags t WHERE t.system_key=$3 ON CONFLICT (site_id,tag_id) DO UPDATE SET position=EXCLUDED.position, role='TERTIARY'`, site.SiteID, tag.Position, tag.TagID)
			} else {
				result, err = tx.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role,assignment_source,position) VALUES ($1::uuid,$3::uuid,'TERTIARY','IMPORTED',$2) ON CONFLICT (site_id,tag_id) DO UPDATE SET position=EXCLUDED.position, role='TERTIARY'`, site.SiteID, tag.Position, tag.TagID)
			}
			if err != nil || result.RowsAffected() != 1 {
				return Counts{}, fmt.Errorf("insert site tertiary tag %q: %w", tag.TagID, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, err)
	}
	return Counts{Sites: len(bundle.Sites), Tags: len(bundle.Tags), SiteTags: bundle.TertiaryCount}, nil
}
