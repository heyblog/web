package taxonomy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func idText(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}
func parseID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(value)
	return id, err
}

func readGraph(ctx context.Context, tx pgx.Tx) (graph, error) {
	q := dbgen.New(tx)
	tags, err := q.ListManagedTags(ctx)
	if err != nil {
		return graph{}, err
	}
	cascades, err := q.ListManagedCascades(ctx)
	if err != nil {
		return graph{}, err
	}
	result := graph{Catalog: Catalog{Tags: make([]Tag, 0, len(tags)), Cascades: make([]Cascade, 0, len(cascades))}, Objects: []object{}}
	aliases, err := q.ListTagIdentityAliases(ctx)
	if err != nil {
		return graph{}, err
	}
	keys := map[string]string{}
	for _, a := range aliases {
		if a.AliasKind == "TAG" && a.SystemKey != nil && *a.SystemKey == "other" {
			keys[idText(a.TagID)] = "other"
		}
	}
	for _, t := range tags {
		result.Tags = append(result.Tags, Tag{ID: idText(t.ID), Name: t.Name, Description: t.Description, Enabled: t.IsEnabled, SystemKey: keys[idText(t.ID)], Roles: []string{}})
	}
	for _, c := range cascades {
		result.Cascades = append(result.Cascades, Cascade{ID: idText(c.ID), Scope: c.Scope, PrimaryID: idText(c.Level1TagID), SecondaryID: idText(c.Level2TagID), Key: c.TaxonomyKey, Enabled: c.IsEnabled, MergedIntoID: idText(c.MergedIntoID)})
	}
	rows, err := tx.Query(ctx, `SELECT 'SITE',s.id::text,s.tag_cascade_id::text,s.revision::text,COALESCE(a.tag_id::text,''),COALESCE(a.role,''),COALESCE(a.assignment_source,''),COALESCE(a.position,0),a.note,COALESCE(a.created_at::text,'') FROM directory.sites s LEFT JOIN directory.site_tags a ON a.site_id=s.id
 UNION ALL SELECT 'ARTICLE',s.id::text,s.tag_cascade_id::text,s.updated_at::text,COALESCE(a.tag_id::text,''),COALESCE(a.role,''),COALESCE(a.assignment_source,''),COALESCE(a.position,0),a.note,COALESCE(a.created_at::text,'') FROM content.articles s LEFT JOIN content.article_tags a ON a.article_id=s.id ORDER BY 1,2,8,5`)
	if err != nil {
		return graph{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var o object
		var a assignment
		if err := rows.Scan(&o.Scope, &o.ID, &o.CascadeID, &o.Version, &a.TagID, &a.Role, &a.Source, &a.Position, &a.Note, &a.CreatedAt); err != nil {
			return graph{}, err
		}
		if len(result.Objects) == 0 || result.Objects[len(result.Objects)-1].ID != o.ID || result.Objects[len(result.Objects)-1].Scope != o.Scope {
			o.Tags = []assignment{}
			result.Objects = append(result.Objects, o)
		}
		if a.TagID != "" {
			i := len(result.Objects) - 1
			result.Objects[i].Tags = append(result.Objects[i].Tags, a)
		}
	}
	if err := rows.Err(); err != nil {
		return graph{}, err
	}
	annotateTagUsage(&result)
	result.Revision, err = digest(result)
	return result, err
}

func digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
func findTag(g graph, id string) Tag {
	for _, t := range g.Tags {
		if t.ID == id {
			return t
		}
	}
	return Tag{}
}
func findCascade(g graph, id string) Cascade {
	for _, c := range g.Cascades {
		if c.ID == id {
			return c
		}
	}
	return Cascade{}
}
func objectKey(o object) string { return o.Scope + ":" + o.ID }
