package taxonomy

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func applyPlan(ctx context.Context, tx pgx.Tx, p changePlan) error {
	// Drop only affected associations first, before any classification changes can trip exclusion triggers.
	for _, o := range p.Graph.Objects {
		if !p.Affected[objectKey(o)] {
			continue
		}
		if err := deleteAssignments(ctx, tx, o); err != nil {
			return err
		}
	}
	// Retire duplicate pairs before updating their endpoints to avoid transient uniqueness conflicts.
	for _, c := range p.Graph.Cascades {
		if c.MergedIntoID != "" {
			if _, err := tx.Exec(ctx, `UPDATE directory.tag_cascades SET merged_into_id=$2::uuid,is_enabled=false WHERE id=$1::uuid`, c.ID, c.MergedIntoID); err != nil {
				return err
			}
		}
	}
	for _, c := range p.Graph.Cascades {
		if _, err := tx.Exec(ctx, `UPDATE directory.tag_cascades SET level1_tag_id=$2::uuid,level2_tag_id=$3::uuid,is_enabled=$4,merged_into_id=NULLIF($5,'')::uuid WHERE id=$1::uuid AND (level1_tag_id<>$2::uuid OR level2_tag_id<>$3::uuid OR is_enabled<>$4 OR merged_into_id IS DISTINCT FROM NULLIF($5,'')::uuid)`, c.ID, c.PrimaryID, c.SecondaryID, c.Enabled, c.MergedIntoID); err != nil {
			return err
		}
	}
	for _, o := range p.Graph.Objects {
		if !p.Affected[objectKey(o)] {
			continue
		}
		if err := writeObject(ctx, tx, o); err != nil {
			return err
		}
	}
	for source, target := range p.Merged {
		if _, err := tx.Exec(ctx, `UPDATE directory.tags SET updated_at=now() WHERE id=$1::uuid`, target); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO directory.tag_identity_aliases(alias_id,tag_id,snapshot) SELECT id,$2::uuid,to_jsonb(t)||jsonb_build_object('labels',(SELECT jsonb_agg(to_jsonb(l)) FROM directory.tag_labels l WHERE l.tag_id=$1::uuid)) FROM directory.tag_dictionary t WHERE id=$1::uuid ON CONFLICT(alias_id) DO UPDATE SET tag_id=EXCLUDED.tag_id`, source, target); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE directory.tag_labels SET tag_id=$2::uuid,updated_at=now() WHERE tag_id=$1::uuid`, source, target); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE directory.tag_identity_aliases SET tag_id=$2::uuid WHERE tag_id=$1::uuid`, source, target); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE directory.tag_slug_aliases SET tag_id=$2::uuid WHERE tag_id=$1::uuid`, source, target); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO directory.tag_slug_aliases(slug,tag_id) SELECT slug,$2::uuid FROM directory.tags WHERE id=$1::uuid ON CONFLICT(slug) DO UPDATE SET tag_id=EXCLUDED.tag_id`, source, target); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM directory.tags WHERE id=$1::uuid`, source); err != nil {
			return err
		}
	}
	ids := []string{}
	for source, target := range p.Merged {
		ids = append(ids, source, target)
	}
	if len(ids) > 0 {
		return invalidateSlugPreviews(ctx, tx, ids)
	}
	return nil
}

func insertCascade(ctx context.Context, tx pgx.Tx, c Cascade) error {
	_, err := tx.Exec(ctx, `INSERT INTO directory.tag_cascades(scope,taxonomy_key,level1_tag_id,level2_tag_id,sort_order,is_enabled) SELECT $1,COALESCE(NULLIF($5,''),'managed/'||uuidv7()::text),$2::uuid,$3::uuid,COALESCE(max(sort_order),0)+1,$4 FROM directory.tag_cascades WHERE scope=$1`, c.Scope, c.PrimaryID, c.SecondaryID, c.Enabled, c.Key)
	return err
}
func deleteAssignments(ctx context.Context, tx pgx.Tx, o object) error {
	archive := `INSERT INTO directory.tag_assignment_archive(scope,object_id,snapshot,reason) SELECT 'SITE',site_id,to_jsonb(a),'taxonomy_change' FROM directory.site_tags a WHERE site_id=$1::uuid`
	if o.Scope == "ARTICLE" {
		archive = `INSERT INTO directory.tag_assignment_archive(scope,object_id,snapshot,reason) SELECT 'ARTICLE',article_id,to_jsonb(a),'taxonomy_change' FROM content.article_tags a WHERE article_id=$1::uuid`
	}
	if _, err := tx.Exec(ctx, archive, o.ID); err != nil {
		return err
	}
	query := `DELETE FROM directory.site_tags WHERE site_id=$1::uuid`
	if o.Scope == "ARTICLE" {
		query = `DELETE FROM content.article_tags WHERE article_id=$1::uuid`
	}
	_, err := tx.Exec(ctx, query, o.ID)
	return err
}
func writeObject(ctx context.Context, tx pgx.Tx, o object) error {
	query := `UPDATE directory.sites SET tag_cascade_id=$2::uuid,primary_label_id=$3::uuid,secondary_label_id=$4::uuid,revision=revision+1 WHERE id=$1::uuid`
	if o.Scope == "ARTICLE" {
		query = `UPDATE content.articles SET tag_cascade_id=$2::uuid,primary_label_id=$3::uuid,secondary_label_id=$4::uuid WHERE id=$1::uuid`
	}
	if _, err := tx.Exec(ctx, query, o.ID, o.CascadeID, o.PrimaryLabelID, o.SecondaryLabelID); err != nil {
		return err
	}
	for _, a := range o.Tags {
		query = `INSERT INTO directory.site_tags(site_id,tag_id,role,assignment_source,position,note,created_at,label_id) VALUES($1::uuid,$2::uuid,$3,$4,NULLIF($5,0),$6,$7::timestamptz,$8::uuid)`
		if o.Scope == "ARTICLE" {
			query = `INSERT INTO content.article_tags(article_id,tag_id,role,assignment_source,position,note,created_at,label_id) VALUES($1::uuid,$2::uuid,$3,$4,NULLIF($5,0),$6,$7::timestamptz,$8::uuid)`
		}
		if _, err := tx.Exec(ctx, query, o.ID, a.TagID, a.Role, a.Source, a.Position, a.Note, a.CreatedAt, a.LabelID); err != nil {
			return err
		}
	}
	return nil
}
