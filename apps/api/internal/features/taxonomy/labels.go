package taxonomy

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Service) CreateLabel(ctx context.Context, id string, input LabelInput) (Catalog, error) {
	return s.editLabels(ctx, id, input.ExpectedRevision, func(tx pgx.Tx, g graph, t Tag) error {
		if strings.TrimSpace(input.Name) == "" {
			return invalid("label_name_required")
		}
		for _, other := range g.Tags {
			if matchesLabel(other, input.Name) {
				return invalid("duplicate_tag_name")
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO directory.tag_labels(tag_id,name,is_enabled) VALUES($1::uuid,$2,$3)`, id, strings.TrimSpace(input.Name), input.Enabled)
		return err
	})
}
func (s *Service) UpdateLabel(ctx context.Context, id, labelID string, input LabelInput) (Catalog, error) {
	return s.editLabels(ctx, id, input.ExpectedRevision, func(tx pgx.Tx, g graph, t Tag) error {
		l := findLabel(t, labelID)
		if l.ID == "" {
			return invalid("label_unavailable")
		}
		if t.DefaultLabelID == labelID && !input.Enabled {
			return invalid("default_label_required")
		}
		if strings.TrimSpace(input.Name) == "" {
			return invalid("label_name_required")
		}
		for _, tag := range g.Tags {
			for _, other := range tag.Labels {
				if other.ID != labelID && strings.EqualFold(strings.TrimSpace(other.Name), strings.TrimSpace(input.Name)) {
					return invalid("duplicate_tag_name")
				}
			}
		}
		_, err := tx.Exec(ctx, `UPDATE directory.tag_labels SET name=$2,is_enabled=$3,updated_at=now() WHERE id=$1::uuid`, labelID, strings.TrimSpace(input.Name), input.Enabled)
		return err
	})
}
func (s *Service) DeleteLabel(ctx context.Context, id, labelID string, input DeleteInput) (Catalog, error) {
	return s.editLabels(ctx, id, input.ExpectedRevision, func(tx pgx.Tx, _ graph, t Tag) error {
		if findLabel(t, labelID).ID == "" {
			return invalid("label_unavailable")
		}
		if t.DefaultLabelID == labelID {
			return invalid("default_label_required")
		}
		var used bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM directory.site_tags WHERE label_id=$1::uuid UNION ALL SELECT 1 FROM content.article_tags WHERE label_id=$1::uuid UNION ALL SELECT 1 FROM directory.sites WHERE primary_label_id=$1::uuid OR secondary_label_id=$1::uuid UNION ALL SELECT 1 FROM content.articles WHERE primary_label_id=$1::uuid OR secondary_label_id=$1::uuid) OR EXISTS(SELECT 1 FROM directory.site_audits WHERE strpos(COALESCE(base_snapshot::text,'')||COALESCE(proposed_snapshot::text,'')||COALESCE(review_draft_snapshot::text,'')||COALESCE(final_snapshot::text,''),$1)>0)`, labelID).Scan(&used)
		if err != nil {
			return err
		}
		if used {
			return invalid("label_in_use")
		}
		_, err = tx.Exec(ctx, `DELETE FROM directory.tag_labels WHERE id=$1::uuid`, labelID)
		return err
	})
}
func (s *Service) DefaultLabel(ctx context.Context, id string, input DefaultLabelInput) (Catalog, error) {
	return s.editLabels(ctx, id, input.ExpectedRevision, func(tx pgx.Tx, _ graph, t Tag) error {
		l := findLabel(t, input.LabelID)
		if l.ID == "" || !l.Enabled {
			return invalid("label_unavailable")
		}
		_, err := tx.Exec(ctx, `UPDATE directory.tags SET default_label_id=$2::uuid WHERE id=$1::uuid`, id, input.LabelID)
		return err
	})
}
func (s *Service) editLabels(ctx context.Context, id, revision string, edit func(pgx.Tx, graph, Tag) error) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(tx pgx.Tx, g graph) error {
		if err := checkRevision(g, revision); err != nil {
			return err
		}
		t := findTag(g, id)
		if t.ID == "" {
			return invalid("tag_unavailable")
		}
		if err := edit(tx, g, t); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE directory.tags SET updated_at=now() WHERE id=$1::uuid`, id); err != nil {
			return err
		}
		if err := invalidateSlugPreviews(ctx, tx, []string{id}); err != nil {
			return err
		}
		fresh, err := readGraph(ctx, tx)
		result = fresh.Catalog
		return err
	})
	return result, err
}
func findLabel(t Tag, id string) Label {
	for _, l := range t.Labels {
		if l.ID == id {
			return l
		}
	}
	return Label{}
}
