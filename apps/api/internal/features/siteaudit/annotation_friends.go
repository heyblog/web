package siteaudit

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"heyblog-api/internal/domain/site"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

func (write annotationWrite) saveFriends(ctx context.Context, source site.Address, current []AnnotationOutgoing) error {
	before := make(map[string]AnnotationFriendInput, len(current))
	for _, link := range current {
		before[link.TargetHost] = link.AnnotationFriendInput
	}
	after := make(map[string]AnnotationFriendInput, len(write.patch.Outgoing))
	for _, link := range write.patch.Outgoing {
		address, err := site.NormalizeAddress(link.TargetURL)
		if err != nil || link.LinkStatus != "ACTIVE" && link.LinkStatus != "INACTIVE" {
			return invalidAnnotation("a friend link is invalid")
		}
		host := address.NormalizedHost
		_, alreadyRecorded := before[host]
		if host == source.NormalizedHost && alreadyRecorded && source.NormalizedHost != write.row.NormalizedHost {
			continue // Moving the source consumes this former external endpoint.
		}
		if host == source.NormalizedHost || host == write.row.NormalizedHost && !alreadyRecorded {
			return invalidAnnotation("a site cannot link to itself")
		}
		if _, duplicate := after[host]; duplicate {
			return invalidAnnotation("only one friend link is allowed per host")
		}
		target, err := write.q.GetSiteByHost(ctx, host)
		switch {
		case err == nil:
			link.TargetURL = annotationURL(target)
		case errors.Is(err, pgx.ErrNoRows):
			link.TargetURL, err = address.HomepageURL()
			if err != nil {
				return invalidAnnotation("a friend link address is invalid")
			}
		default:
			return err
		}
		after[host] = link
	}
	for host, link := range before {
		if _, found := after[host]; !found && host != source.NormalizedHost {
			link.LinkStatus = "INACTIVE"
			after[host] = link
		}
	}
	for _, host := range slices.Sorted(maps.Keys(after)) {
		link := after[host]
		if before[host] == link {
			continue
		}
		if err := write.q.UpsertFriendLink(ctx, dbgen.UpsertFriendLinkParams{
			PSourceSiteID: write.row.ID, PTargetUrl: link.TargetURL, PTargetHost: host, PStatus: link.LinkStatus,
		}); err != nil {
			var dbError *pgconn.PgError
			if errors.As(err, &dbError) && (dbError.Code == "40P01" ||
				dbError.Code == "P0001" && strings.HasPrefix(dbError.Message, "registered friend-link target URL")) {
				return annotationConflict()
			}
			return err
		}
	}
	return nil
}
