package taxonomy

import (
	"context"
	"strings"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Create(ctx context.Context, input CreateInput) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(tx pgx.Tx, g graph) error {
		if err := checkRevision(g, input.ExpectedRevision); err != nil {
			return err
		}
		for _, tag := range g.Tags {
			if strings.EqualFold(strings.TrimSpace(input.Name), strings.TrimSpace(tag.Name)) {
				if tag.Enabled {
					return invalid("duplicate_tag_name")
				}
				id, err := parseID(tag.ID)
				if err != nil {
					return err
				}
				if err = dbgen.New(tx).EnableCanonicalTag(ctx, id); err != nil {
					return err
				}
				fresh, err := readGraph(ctx, tx)
				result = fresh.Catalog
				return err
			}
		}
		q := dbgen.New(tx)
		if err := checkSlug(ctx, q, input.Slug, ""); err != nil {
			return err
		}
		_, err := q.CreateManagedTag(ctx, dbgen.CreateManagedTagParams{Name: strings.TrimSpace(input.Name), Slug: input.Slug, Description: strings.TrimSpace(input.Description)})
		if err != nil {
			return err
		}
		fresh, err := readGraph(ctx, tx)
		result = fresh.Catalog
		return err
	})
	return result, err
}
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(tx pgx.Tx, g graph) error {
		if err := checkRevision(g, input.ExpectedRevision); err != nil {
			return err
		}
		tag := findTag(g, id)
		if tag.ID == "" {
			return invalid("tag_unavailable")
		}
		if protectedTag(tag) && !input.Enabled {
			return invalid("fallback_protected")
		}
		q := dbgen.New(tx)
		if err := checkSlug(ctx, q, input.Slug, id); err != nil {
			return err
		}
		uuid, err := parseID(id)
		if err != nil {
			return err
		}
		if tag.Slug != input.Slug {
			if err := q.ReserveTaxonomySlug(ctx, dbgen.ReserveTaxonomySlugParams{Slug: tag.Slug, TagID: uuid}); err != nil {
				return err
			}
		}
		if err := q.UpdateManagedTag(ctx, dbgen.UpdateManagedTagParams{ID: uuid, Name: strings.TrimSpace(input.Name), Slug: input.Slug, Description: strings.TrimSpace(input.Description), IsEnabled: input.Enabled}); err != nil {
			return err
		}
		if !input.Enabled {
			if _, err = tx.Exec(ctx, `UPDATE directory.tag_cascades SET is_enabled=false WHERE level1_tag_id=$1 OR level2_tag_id=$1`, uuid); err != nil {
				return err
			}
		}
		fresh, err := readGraph(ctx, tx)
		result = fresh.Catalog
		return err
	})
	return result, err
}
func (s *Service) Delete(ctx context.Context, id string, input DeleteInput) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(tx pgx.Tx, g graph) error {
		if err := checkRevision(g, input.ExpectedRevision); err != nil {
			return err
		}
		t := findTag(g, id)
		if t.ID == "" || protectedTag(t) || t.SiteCount > 0 || t.ArticleCount > 0 {
			return invalid("tag_in_use")
		}
		ids := []string{id}
		for _, c := range g.Cascades {
			if c.PrimaryID == id || c.SecondaryID == id {
				if c.MergedIntoID != "" {
					return invalid("tag_has_aliases")
				}
				ids = append(ids, c.ID)
			}
		}
		var historical bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM directory.site_audits a CROSS JOIN unnest($1::text[]) AS reference WHERE strpos(COALESCE(a.base_snapshot::text,'')||COALESCE(a.proposed_snapshot::text,'')||COALESCE(a.review_draft_snapshot::text,'')||COALESCE(a.final_snapshot::text,''),reference)>0)`, ids).Scan(&historical); err != nil {
			return err
		}
		if historical {
			return invalid("tag_in_audit_history")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM directory.tag_cascades WHERE level1_tag_id=$1::uuid OR level2_tag_id=$1::uuid`, id); err != nil {
			return err
		}
		uuid, err := parseID(id)
		if err != nil {
			return err
		}
		if err = dbgen.New(tx).DeleteManagedTag(ctx, uuid); err != nil {
			return err
		}
		fresh, err := readGraph(ctx, tx)
		result = fresh.Catalog
		return err
	})
	return result, err
}
func invalid(code string) error {
	return apperror.New(apperror.KindValidation, code, "the requested taxonomy change is invalid")
}
func checkSlug(ctx context.Context, q *dbgen.Queries, slug, id string) error {
	rows, err := q.TaxonomySlugOwner(ctx, slug)
	if err != nil {
		return err
	}
	for _, owner := range rows {
		if idText(owner) != id {
			return apperror.New(apperror.KindConflict, "slug_conflict", "the slug is already reserved")
		}
	}
	return nil
}
