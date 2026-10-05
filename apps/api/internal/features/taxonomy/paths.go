package taxonomy

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *Service) CreateCascade(ctx context.Context, input CreateCascadeInput) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(tx pgx.Tx, g graph) error {
		if err := checkRevision(g, input.ExpectedRevision); err != nil {
			return err
		}
		if input.Scope != "SITE" && input.Scope != "ARTICLE" {
			return invalid("invalid_scope")
		}
		p, c := findTag(g, input.PrimaryID), findTag(g, input.SecondaryID)
		if p.ID == "" || c.ID == "" || !p.Enabled || !c.Enabled {
			return invalid("path_tags_unavailable")
		}
		if err := insertCascade(ctx, tx, Cascade{Scope: input.Scope, PrimaryID: p.ID, SecondaryID: c.ID, Key: input.Key, Enabled: true}); err != nil {
			return err
		}
		fresh, err := readGraph(ctx, tx)
		result = fresh.Catalog
		return err
	})
	return result, err
}
