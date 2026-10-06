package taxonomy

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Invalidate records inside the same taxonomy transaction, fencing outstanding workers.
func invalidateSlugPreviews(ctx context.Context, tx pgx.Tx, ids []string) error {
	_, err := tx.Exec(ctx, `UPDATE directory.slug_generation_jobs j SET items=(
 SELECT jsonb_agg(CASE WHEN item->>'state' IN ('pending','running','ready','needs_confirmation') AND
 (item->>'tag_id'=ANY($1::text[]) OR EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(item->'conflicts')='array' THEN item->'conflicts' ELSE '[]'::jsonb END) c WHERE c->>'id'=ANY($1::text[])))
 THEN item||jsonb_build_object('state','stale','error_code','slug_job_taxonomy_changed') ELSE item END ORDER BY ordinal)
 FROM jsonb_array_elements(j.items) WITH ORDINALITY AS records(item,ordinal)),revision=revision+1,updated_at=now()
 WHERE status NOT IN ('cancelled','completed') AND EXISTS(
 SELECT 1 FROM jsonb_array_elements(j.items) item WHERE item->>'state' IN ('pending','running','ready','needs_confirmation') AND
 (item->>'tag_id'=ANY($1::text[]) OR EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(item->'conflicts')='array' THEN item->'conflicts' ELSE '[]'::jsonb END) c WHERE c->>'id'=ANY($1::text[]))))`, ids)
	return err
}
