package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type AccountFriend struct {
	TargetShortID string `json:"target_short_id"`
	TargetName    string `json:"target_name"`
	TargetHost    string `json:"target_host"`
	TargetURL     string `json:"target_url"`
	LinkStatus    string `json:"link_status"`
	IsReciprocal  bool   `json:"is_reciprocal"`
}

type AccountFriendRequest struct {
	ID         string `json:"id"`
	AuditID    string `json:"audit_id"`
	TargetName string `json:"target_name"`
	TargetURL  string `json:"target_url"`
	Status     string `json:"status"`
}

type AccountFriends struct {
	Items   []AccountFriend        `json:"items"`
	Pending []AccountFriendRequest `json:"pending"`
}

func (repository *Repository) accountFriends(ctx context.Context, user string, shortID string) (AccountFriends, error) {
	snapshot, err := repository.accountSite(ctx, user, shortID)
	if err != nil {
		return AccountFriends{}, err
	}
	siteID, err := parseUUID(snapshot.SiteID)
	if err != nil {
		return AccountFriends{}, err
	}
	userID, err := parseUUID(user)
	if err != nil {
		return AccountFriends{}, err
	}
	links, err := repository.queries.ListOwnerFriendLinks(ctx, siteID)
	if err != nil {
		return AccountFriends{}, err
	}
	result := AccountFriends{Items: make([]AccountFriend, 0, len(links)), Pending: []AccountFriendRequest{}}
	for _, link := range links {
		result.Items = append(result.Items, AccountFriend{TargetShortID: stringValue(link.TargetShortID), TargetName: stringValue(link.TargetName), TargetHost: link.TargetHost, TargetURL: link.TargetUrl, LinkStatus: link.LinkStatus, IsReciprocal: link.IsReciprocal})
	}
	requests, err := repository.queries.ListOwnerFriendRequests(ctx, dbgen.ListOwnerFriendRequestsParams{SourceSiteID: siteID, UserID: userID})
	if err != nil {
		return AccountFriends{}, err
	}
	for _, request := range requests {
		id, err := uuidString(request.ID)
		if err != nil {
			return AccountFriends{}, err
		}
		auditID, err := uuidString(request.AuditID)
		if err != nil {
			return AccountFriends{}, err
		}
		result.Pending = append(result.Pending, AccountFriendRequest{ID: id, AuditID: auditID, TargetName: request.TargetName, TargetURL: request.TargetUrl, Status: request.Status})
	}
	return result, nil
}

func (repository *Repository) setAccountFriend(ctx context.Context, user string, shortID, targetShortID string, active bool) error {
	if shortID == targetShortID {
		return newServiceError("friend_link_self", http.StatusUnprocessableEntity, "a site cannot link to itself")
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin friend update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repository.queries.WithTx(tx)
	if _, err := queries.LockFriendEndpointSites(ctx, dbgen.LockFriendEndpointSitesParams{SourceShortID: shortID, TargetShortID: targetShortID}); err != nil {
		return err
	}
	userID, err := parseUUID(user)
	if err != nil {
		return err
	}
	owner, err := queries.LockCurrentSiteOwner(ctx, dbgen.LockCurrentSiteOwnerParams{ShortID: shortID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("site_ownership_required", http.StatusForbidden, "active site ownership is required")
	}
	if err != nil {
		return err
	}
	target, err := queries.GetSiteByShortID(ctx, targetShortID)
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("site_not_found", http.StatusNotFound, "the friend site was not found")
	}
	if err != nil {
		return err
	}
	if active && target.Visibility == "REMOVED" {
		return newServiceError("site_removed", http.StatusConflict, "a removed site cannot be added as a friend")
	}
	status := "INACTIVE"
	if active {
		status = "ACTIVE"
	}
	if err := queries.SetRegisteredFriendLink(ctx, dbgen.SetRegisteredFriendLinkParams{PSourceSiteID: owner.SiteID, PTargetUrl: target.Scheme + "://" + target.NormalizedHost + target.BasePath, PTargetHost: target.NormalizedHost, PStatus: status}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *Repository) cancelAccountFriend(ctx context.Context, user string, shortID, requestID string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repository.queries.WithTx(tx)
	userID, err := parseUUID(user)
	if err != nil {
		return err
	}
	id, err := parseUUID(requestID)
	if err != nil {
		return newServiceError("friend_request_not_found", http.StatusNotFound, "the pending friend request was not found")
	}
	auditID, err := queries.GetAccountFriendRequestAudit(ctx, dbgen.GetAccountFriendRequestAuditParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("friend_request_not_found", http.StatusNotFound, "the pending friend request was not found")
	}
	if err != nil {
		return err
	}
	// Match approval and recommendation lock order: audit, source site, ownership, request.
	if _, err := queries.LockSiteAuditByID(ctx, auditID); err != nil {
		return err
	}
	owner, err := queries.LockCurrentSiteOwner(ctx, dbgen.LockCurrentSiteOwnerParams{ShortID: shortID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("site_ownership_required", http.StatusForbidden, "active site ownership is required")
	}
	if err != nil {
		return err
	}
	_, err = queries.CancelOwnerFriendRequest(ctx, dbgen.CancelOwnerFriendRequestParams{ID: id, SourceSiteID: owner.SiteID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("friend_request_not_found", http.StatusNotFound, "the pending friend request was not found")
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *Repository) removeAccountFriend(ctx context.Context, user, shortID, targetHost string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := repository.queries.WithTx(tx)
	userID, err := parseUUID(user)
	if err != nil {
		return err
	}
	owner, err := queries.LockCurrentSiteOwner(ctx, dbgen.LockCurrentSiteOwnerParams{ShortID: shortID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return newServiceError("site_ownership_required", http.StatusForbidden, "active site ownership is required")
	}
	if err != nil {
		return err
	}
	removed, err := queries.RemoveOwnerFriendLink(ctx, dbgen.RemoveOwnerFriendLinkParams{PSourceSiteID: owner.SiteID, PTargetHost: targetHost})
	if err != nil {
		return err
	}
	if !removed {
		return newServiceError("friend_link_not_found", http.StatusNotFound, "the active friend link was not found")
	}
	return tx.Commit(ctx)
}
