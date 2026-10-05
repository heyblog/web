package dataimport

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type incrementalPair struct {
	SourceID   string
	TargetHost string
}

func importUUIDString(value pgtype.UUID) string {
	id := value.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
}

func resolveIncremental(plan Plan, stored []dbgen.ListIncrementalSitesRow, pairs []dbgen.ListIncrementalFriendPairsRow, generate func() (string, error)) (Plan, Counts, error) {
	byHost := make(map[string]dbgen.ListIncrementalSitesRow, len(stored)+len(plan.Sites))
	usedIDs := make(map[string]string, len(stored))
	usedShortIDs := make(map[string]struct{}, len(stored)+len(plan.Sites))
	for _, row := range stored {
		byHost[row.NormalizedHost] = row
		usedIDs[importUUIDString(row.ID)] = row.NormalizedHost
		usedShortIDs[row.ShortID] = struct{}{}
	}
	resolved := Plan{Sources: plan.Sources}
	counts := Counts{}
	idMapping := make(map[string]string, len(plan.Sites))
	created := make(map[string]struct{})
	for _, row := range plan.Sites {
		if existing, ok := byHost[row.NormalizedHost]; ok {
			idMapping[row.ID] = importUUIDString(existing.ID)
			counts.MatchedSites++
			continue
		}
		if host, exists := usedIDs[row.ID]; exists {
			return Plan{}, Counts{}, fmt.Errorf("incoming site ID already belongs to host %q", host)
		}
		if _, exists := usedShortIDs[row.ShortID]; exists {
			value, err := uniqueShortID(generate, usedShortIDs)
			if err != nil {
				return Plan{}, Counts{}, err
			}
			row.ShortID = value
		} else {
			usedShortIDs[row.ShortID] = struct{}{}
		}
		resolved.Sites = append(resolved.Sites, row)
		counts.Sites++
		idMapping[row.ID] = row.ID
		created[row.ID] = struct{}{}
		byHost[row.NormalizedHost] = dbgen.ListIncrementalSitesRow{
			ID: mustUUID(row.ID), ShortID: row.ShortID, NormalizedHost: row.NormalizedHost, Scheme: row.Scheme, BasePath: row.BasePath,
		}
	}
	for _, feed := range plan.Feeds {
		if _, isNew := created[feed.SiteID]; isNew {
			resolved.Feeds = append(resolved.Feeds, feed)
			counts.Feeds++
		}
	}
	for _, origin := range plan.Origins {
		origin.SiteID = idMapping[origin.SiteID]
		resolved.Origins = append(resolved.Origins, origin)
	}
	existingPairs := make(map[incrementalPair]struct{}, len(pairs))
	for _, pair := range pairs {
		existingPairs[incrementalPair{SourceID: importUUIDString(pair.SourceSiteID), TargetHost: pair.TargetHost}] = struct{}{}
	}
	for _, link := range plan.FriendLinks {
		link.SourceSiteID = idMapping[link.SourceSiteID]
		if _, exists := existingPairs[incrementalPair{SourceID: link.SourceSiteID, TargetHost: link.TargetHost}]; exists {
			counts.ExistingFriendLinks++
			continue
		}
		target := byHost[link.TargetHost]
		link.TargetURL = target.Scheme + "://" + target.NormalizedHost + target.BasePath
		resolved.FriendLinks = append(resolved.FriendLinks, link)
		counts.FriendLinks++
	}
	return resolved, counts, nil
}
