package siteaudit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"heyblog-api/internal/domain/site"
)

func (service *Service) Query(ctx context.Context, lookupToken string) (PublicAuditResult, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(lookupToken))
	if err != nil || len(decoded) != 32 {
		return PublicAuditResult{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit lookup credential was not found")
	}
	digest := sha256.Sum256(decoded)
	audit, err := service.repository.LookupAudit(ctx, digest[:])
	if err != nil {
		return PublicAuditResult{}, err
	}
	return PublicAuditResult{Action: audit.Action, Status: audit.Status, ShortID: publicAuditShortID(audit), ReviewerComment: audit.ReviewerComment, ReviewedAt: audit.ReviewedAt, CreatedAt: audit.CreatedAt}, nil
}

func (service *Service) ResolveSite(ctx context.Context, shortID string) (Snapshot, error) {
	if err := site.ValidateShortID(shortID); err != nil {
		return Snapshot{}, newServiceError("site_not_found", 404, "the target site was not found")
	}
	snapshot, err := service.repository.ResolveSite(ctx, shortID)
	if err != nil {
		return Snapshot{}, err
	}
	return publicSiteSnapshot(snapshot), nil
}

func (service *Service) Options(ctx context.Context) (SubmissionOptions, error) {
	return service.repository.Options(ctx)
}

func (service *Service) SearchSites(ctx context.Context, query string) ([]SiteSearchResult, error) {
	return service.repository.SearchSites(ctx, query)
}

func (service *Service) CheckSiteAvailability(ctx context.Context, rawURL string) (SiteAvailability, error) {
	address, err := site.NormalizeAddress(rawURL)
	if err != nil {
		return SiteAvailability{}, newServiceError("invalid_site_address", http.StatusUnprocessableEntity, "the site address is invalid")
	}
	existing, err := service.repository.ExistingSiteForHost(ctx, address.NormalizedHost)
	if err != nil {
		return SiteAvailability{}, err
	}
	return SiteAvailability{Available: existing == nil, ExistingSite: existing}, nil
}

func (service *Service) ensureCreateAddressAvailable(ctx context.Context, action Action, normalizedHost string) error {
	if action != ActionCreate {
		return nil
	}
	existing, err := service.repository.ExistingSiteForHost(ctx, normalizedHost)
	if err != nil {
		return err
	}
	if existing != nil {
		return siteAddressConflictError()
	}
	return nil
}

func publicSiteSnapshot(snapshot Snapshot) Snapshot {
	snapshot.SiteID = ""
	return snapshot
}

func publicAuditShortID(audit Audit) string {
	if audit.FinalSnapshot.ShortID != "" {
		return audit.FinalSnapshot.ShortID
	}
	if audit.ProposedSnapshot.ShortID != "" {
		return audit.ProposedSnapshot.ShortID
	}
	return audit.BaseSnapshot.ShortID
}
