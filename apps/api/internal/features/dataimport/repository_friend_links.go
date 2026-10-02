package dataimport

import (
	"context"
	"encoding/json"
	"fmt"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type friendLinkInsertRow struct {
	SourceSiteID string `json:"source_site_id"`
	TargetURL    string `json:"target_url"`
	TargetHost   string `json:"target_host"`
}

func insertFriendLinks(ctx context.Context, queries *dbgen.Queries, plan Plan) error {
	for start := 0; start < len(plan.FriendLinks); start += friendLinkBatchSize {
		end := min(start+friendLinkBatchSize, len(plan.FriendLinks))
		batch := make([]friendLinkInsertRow, 0, end-start)
		for _, row := range plan.FriendLinks[start:end] {
			batch = append(batch, friendLinkInsertRow(row))
		}
		encoded, err := json.Marshal(batch)
		if err != nil {
			return fmt.Errorf("encode friend links batch starting at %d: %w", start, err)
		}
		if err := queries.InsertFriendLinks(ctx, encoded); err != nil {
			return fmt.Errorf("insert friend links batch starting at %d: %w", start, err)
		}
	}
	return nil
}
