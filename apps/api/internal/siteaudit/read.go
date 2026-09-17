package siteaudit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
)

const privateProgramNormalizedName = "其他"

func (service *Service) Options(ctx context.Context) (SubmissionOptions, error) {
	cascades, err := service.repository.queries.ListEnabledSiteTagCascades(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission tag cascades: %w", err)
	}
	tags, err := service.repository.queries.ListEnabledTags(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission tag options: %w", err)
	}
	components, err := service.repository.queries.ListEnabledSoftwareComponents(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission component options: %w", err)
	}
	dependencies, err := service.repository.queries.ListEnabledSoftwareComponentDependencies(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission program dependencies: %w", err)
	}
	options := SubmissionOptions{
		Tags: make([]Option, 0, len(tags)+len(cascades)*2), Cascades: make([]CascadeOption, 0, len(cascades)), Components: make([]ComponentOption, 0, len(components)),
		ProgramDependencies: make([]ProgramDependencyOption, 0, len(dependencies)),
	}
	seenLevel1 := make(map[string]struct{})
	for _, cascade := range cascades {
		cascadeID, idErr := uuidString(cascade.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level1ID, idErr := uuidString(cascade.Level1ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level2ID, idErr := uuidString(cascade.Level2ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level1 := Option{ID: level1ID, Name: cascade.Level1Name, Level: 1}
		level2 := Option{ID: level2ID, Name: cascade.Level2Name, Level: 2, ParentID: level1ID}
		if _, exists := seenLevel1[level1ID]; !exists {
			options.Tags = append(options.Tags, level1)
			seenLevel1[level1ID] = struct{}{}
		}
		options.Tags = append(options.Tags, level2)
		options.Cascades = append(options.Cascades, CascadeOption{ID: cascadeID, TaxonomyKey: cascade.TaxonomyKey, Level1: level1, Level2: level2})
	}
	for _, tag := range tags {
		id, idErr := uuidString(tag.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		options.Tags = append(options.Tags, Option{ID: id, Name: tag.Name, Level: 3})
	}
	for _, component := range components {
		id, idErr := uuidString(component.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		options.Components = append(options.Components, ComponentOption{ID: id, Name: component.Name, HomepageURL: stringValue(component.HomepageUrl), RepositoryURL: stringValue(component.RepositoryUrl), IsOpenSource: component.IsOpenSource})
		if component.NormalizedName == privateProgramNormalizedName {
			options.PrivateProgramID = id
		}
	}
	for _, dependency := range dependencies {
		programID, idErr := uuidString(dependency.ComponentID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		componentID, idErr := uuidString(dependency.DependencyComponentID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		options.ProgramDependencies = append(options.ProgramDependencies, ProgramDependencyOption{ProgramID: programID, ComponentID: componentID, Role: dependency.Role})
	}
	return options, nil
}

func (service *Service) SearchSites(ctx context.Context, query string) ([]SiteSearchResult, error) {
	rows, err := service.repository.queries.SearchSitesForSubmission(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search sites for submission: %w", err)
	}
	results := make([]SiteSearchResult, 0, len(rows))
	for _, row := range rows {
		result, mapErr := siteSearchResult(row)
		if mapErr != nil {
			return nil, mapErr
		}
		results = append(results, result)
	}
	return results, nil
}

func (service *Service) CheckSiteAvailability(ctx context.Context, rawURL string) (SiteAvailability, error) {
	address, err := site.NormalizeAddress(rawURL)
	if err != nil {
		return SiteAvailability{}, newServiceError("invalid_site_address", http.StatusUnprocessableEntity, "the site address is invalid")
	}
	existing, err := service.existingSiteForHost(ctx, address.NormalizedHost)
	if err != nil {
		return SiteAvailability{}, err
	}
	return SiteAvailability{Available: existing == nil, ExistingSite: existing}, nil
}

func (service *Service) ensureCreateAddressAvailable(ctx context.Context, action Action, normalizedHost string) error {
	if action != ActionCreate {
		return nil
	}
	existing, err := service.existingSiteForHost(ctx, normalizedHost)
	if err != nil {
		return err
	}
	if existing != nil {
		return siteAddressConflictError()
	}
	return nil
}

func (service *Service) existingSiteForHost(ctx context.Context, normalizedHost string) (*SiteSearchResult, error) {
	row, err := service.repository.queries.GetSiteByHost(ctx, normalizedHost)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find site by normalized host: %w", err)
	}
	result, err := siteSearchResult(row)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func siteSearchResult(row dbgen.DirectorySite) (SiteSearchResult, error) {
	urlValue, err := (site.Address{Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath}).HomepageURL()
	if err != nil {
		return SiteSearchResult{}, fmt.Errorf("map searched site address: %w", err)
	}
	return SiteSearchResult{ShortID: row.ShortID, Name: row.Name, URL: urlValue, Visibility: row.Visibility}, nil
}

func (service *Service) ResolveSite(ctx context.Context, shortID string) (Snapshot, error) {
	if err := site.ValidateShortID(shortID); err != nil {
		return Snapshot{}, newServiceError("site_not_found", 404, "the target site was not found")
	}
	row, err := service.repository.queries.GetSiteByShortID(ctx, shortID)
	if err != nil {
		if isNotFound(err) {
			return Snapshot{}, newServiceError("site_not_found", 404, "the target site was not found")
		}
		return Snapshot{}, fmt.Errorf("get site by short ID: %w", err)
	}
	snapshot, err := loadSnapshot(ctx, service.repository.queries, row.ID)
	if err != nil {
		if isNotFound(err) {
			return Snapshot{}, newServiceError("site_not_found", 404, "the target site was not found")
		}
		return Snapshot{}, err
	}
	return publicSiteSnapshot(snapshot), nil
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

func (service *Service) ListAudits(ctx context.Context, status *Status, action *Action, page, pageSize int32) (AuditPage, error) {
	statusValue, actionValue := optionalFilters(status, action)
	offset := (page - 1) * pageSize
	rows, err := service.repository.queries.ListSiteAuditsForManagement(ctx, dbgen.ListSiteAuditsForManagementParams{Status: statusValue, Action: actionValue, PageOffset: offset, PageSize: pageSize})
	if err != nil {
		return AuditPage{}, fmt.Errorf("list site audits: %w", err)
	}
	total, err := service.repository.queries.CountSiteAuditsForManagement(ctx, dbgen.CountSiteAuditsForManagementParams{Status: statusValue, Action: actionValue})
	if err != nil {
		return AuditPage{}, fmt.Errorf("count site audits: %w", err)
	}
	items := make([]AuditListItem, 0, len(rows))
	for _, row := range rows {
		item, mapErr := mapAuditListItem(row)
		if mapErr != nil {
			return AuditPage{}, mapErr
		}
		items = append(items, item)
	}
	return AuditPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: int32(math.Ceil(float64(total) / float64(pageSize)))}, nil
}

func (service *Service) AuditDetail(ctx context.Context, auditID string) (Audit, error) {
	id, err := parseUUID(auditID)
	if err != nil {
		return Audit{}, newServiceError("audit_not_found", 404, "the audit was not found")
	}
	row, err := service.repository.queries.GetSiteAuditByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return Audit{}, newServiceError("audit_not_found", 404, "the audit was not found")
		}
		return Audit{}, fmt.Errorf("get site audit: %w", err)
	}
	audit, err := auditFromRow(row)
	if err != nil {
		return Audit{}, err
	}
	current := audit.BaseSnapshot
	if row.SiteID.Valid {
		current, err = loadSnapshot(ctx, service.repository.queries, row.SiteID)
		if err != nil {
			return Audit{}, err
		}
		audit.HasCurrentSnapshot = true
	}
	audit.CurrentSnapshot = current
	audit.EffectiveSnapshot, _ = MergeRequestedSnapshot(audit.BaseSnapshot, audit.ProposedSnapshot, current)
	var correction *Snapshot
	if audit.Status == StatusApproved {
		correction = &audit.FinalSnapshot
	} else if audit.ReviewDraftSnapshot != nil {
		correction = audit.ReviewDraftSnapshot
	}
	audit.Diff = BuildDiffViews(audit.BaseSnapshot, audit.ProposedSnapshot, current, correction)
	return audit, nil
}

func optionalFilters(status *Status, action *Action) (*string, *string) {
	var statusValue, actionValue *string
	if status != nil {
		value := string(*status)
		statusValue = &value
	}
	if action != nil {
		value := string(*action)
		actionValue = &value
	}
	return statusValue, actionValue
}

func mapAuditListItem(row dbgen.ListSiteAuditsForManagementRow) (AuditListItem, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return AuditListItem{}, err
	}
	proposed := Snapshot{}
	if err := decodeSnapshot(row.ProposedSnapshot, &proposed); err != nil {
		return AuditListItem{}, fmt.Errorf("decode audit list snapshot: %w", err)
	}
	item := AuditListItem{ID: id, Action: Action(row.Action), Status: Status(row.Status), SiteName: proposed.Name, SiteAddress: snapshotAddress(proposed), SubmitterName: stringValue(row.SubmitterName), SubmitterEmail: stringValue(row.SubmitterEmail), ReviewedAt: timestampPointer(row.ReviewedAt), CreatedAt: row.CreatedAt.Time}
	if row.SiteID.Valid {
		item.SiteID, err = uuidString(row.SiteID)
	}
	return item, err
}
