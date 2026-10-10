package dataimport

import (
	"context"
	"errors"
	"fmt"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ImportTaxonomy preserves dictionary management and resolves historical source identities.
func (repository *Repository) ImportTaxonomy(ctx context.Context, bundle TagTaxonomyBundle) (Counts, error) {
	if repository.pool == nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, errors.New("database pool is unavailable"))
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = dbgen.New(tx).LockTaxonomy(ctx); err != nil {
		return Counts{}, err
	}
	for _, tag := range bundle.Tags {
		if tag.Source == "SQLITE" {
			continue
		}
		id, err := parseUUIDText(tag.TagID)
		if err != nil {
			return Counts{}, fmt.Errorf("legacy taxonomy tag: %w", err)
		}
		if _, lookupErr := dbgen.New(tx).GetCanonicalTag(ctx, id); lookupErr == nil {
			continue
		} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return Counts{}, lookupErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO directory.tags(id,name,description,is_enabled) SELECT $1,$2,COALESCE($3,''),true WHERE NOT EXISTS(SELECT 1 FROM directory.tags WHERE normalized_name=lower(btrim($2)))`, id, tag.Name, tag.Description); err != nil {
			return Counts{}, fmt.Errorf("insert imported tag: %w", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO directory.tag_identity_aliases(alias_kind,alias_id,tag_id,snapshot) SELECT 'TAG',$1,id,jsonb_build_object('id',$1::text,'name',$2::text) FROM directory.tags WHERE normalized_name=lower(btrim($2)) ON CONFLICT(alias_kind,alias_id) DO NOTHING`, id, tag.Name); err != nil {
			return Counts{}, err
		}
	}
	for _, site := range bundle.Sites {
		if err := importSiteTaxonomy(ctx, tx, site); err != nil {
			return Counts{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Counts{}, errors.Join(ErrDependencyUnavailable, err)
	}
	return Counts{Sites: len(bundle.Sites), Tags: len(bundle.Tags), SiteTags: bundle.TertiaryCount}, nil
}

func importSiteTaxonomy(ctx context.Context, tx pgx.Tx, site SiteTagTaxonomyMigration) error {
	q := dbgen.New(tx)
	var pathID pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM directory.tag_cascades WHERE scope='SITE' AND taxonomy_key=$1`, site.CascadeKey).Scan(&pathID); err != nil {
		return fmt.Errorf("find imported classification: %w", err)
	}
	path, err := q.GetCanonicalCascade(ctx, pathID)
	if err != nil {
		return err
	}
	if !path.IsEnabled {
		return errors.New("imported classification is disabled; update the import mapping")
	}
	type selectedTag struct{ id pgtype.UUID }
	assignments := make([]selectedTag, 0, len(site.TertiaryTags))
	seen := map[pgtype.UUID]bool{}
	for _, tag := range site.TertiaryTags {
		var id pgtype.UUID
		if tag.Source == "SQLITE" {
			err = tx.QueryRow(ctx, `SELECT tag_id FROM directory.tag_identity_aliases WHERE alias_kind='TAG' AND system_key=$1`, tag.TagID).Scan(&id)
		} else {
			id, err = parseUUIDText(tag.TagID)
		}
		if err != nil {
			return fmt.Errorf("find imported tag: %w", err)
		}
		canonical, err := q.GetCanonicalTag(ctx, id)
		if err != nil {
			return err
		}
		if !canonical.IsEnabled {
			return errors.New("imported tag is disabled; update the import mapping")
		}
		if seen[canonical.ID] || canonical.ID == path.Level1TagID || canonical.ID == path.Level2TagID {
			continue
		}
		seen[canonical.ID] = true
		assignments = append(assignments, selectedTag{id: canonical.ID})
	}
	if _, err = tx.Exec(ctx, `DELETE FROM directory.site_tags WHERE site_id=$1::uuid AND role='TERTIARY'`, site.SiteID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE directory.sites SET tag_cascade_id=$2,revision=revision+1 WHERE id=$1::uuid`, site.SiteID, path.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("imported taxonomy site does not exist")
	}
	for i, a := range assignments {
		if _, err = tx.Exec(ctx, `INSERT INTO directory.site_tags(site_id,tag_id,role,assignment_source,position) VALUES($1::uuid,$2,'TERTIARY','IMPORTED',$3)`, site.SiteID, a.id, i+1); err != nil {
			return fmt.Errorf("assign imported tag: %w", err)
		}
	}
	return nil
}
