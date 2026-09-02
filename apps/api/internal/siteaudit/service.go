package siteaudit

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/auth"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/mail"
)

type Service struct {
	repository *Repository
	auth       *auth.Service
	newShortID func() (string, error)
	mailer     *mail.SubmissionMailer
	logger     *slog.Logger
}

type Dependencies struct {
	Repository *Repository
	Auth       *auth.Service
	NewShortID func() (string, error)
	Mailer     *mail.SubmissionMailer
	Logger     *slog.Logger
}

func NewService(dependencies Dependencies) *Service {
	return &Service{repository: dependencies.Repository, auth: dependencies.Auth, newShortID: dependencies.NewShortID, mailer: dependencies.Mailer, logger: dependencies.Logger}
}

func (service *Service) Submit(
	ctx context.Context,
	action Action,
	targetShortID string,
	input SubmissionInput,
) (SubmissionResult, error) {
	normalized, err := NormalizeSubmission(action, input)
	if err != nil {
		return SubmissionResult{}, err
	}
	base := Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"}
	var targetID pgtype.UUID
	if action != ActionCreate {
		if err := site.ValidateShortID(targetShortID); err != nil {
			return SubmissionResult{}, fmt.Errorf("%w: target site short ID is invalid", ErrInvalidSubmission)
		}
		targetSite, err := service.repository.queries.GetSiteByShortID(ctx, targetShortID)
		if err != nil {
			if isNotFound(err) {
				return SubmissionResult{}, newServiceError("site_not_found", http.StatusNotFound, "the target site was not found")
			}
			return SubmissionResult{}, fmt.Errorf("get target site by short ID: %w", err)
		}
		targetID = targetSite.ID
		base, err = loadSnapshot(ctx, service.repository.queries, targetID)
		if err != nil {
			if isNotFound(err) {
				return SubmissionResult{}, newServiceError("site_not_found", http.StatusNotFound, "the target site was not found")
			}
			return SubmissionResult{}, err
		}
	}
	proposed, err := proposedForAction(action, normalized, base)
	if err != nil {
		return SubmissionResult{}, err
	}
	if action == ActionCreate || action == ActionUpdate {
		proposed, err = service.prepareSubmissionTaxonomy(ctx, proposed)
		if err != nil {
			return SubmissionResult{}, err
		}
	}
	if action == ActionUpdate && len(buildSnapshotDiff(base, proposed)) == 0 {
		return SubmissionResult{}, newServiceError("submission_no_changes", http.StatusUnprocessableEntity, "the update does not change the site")
	}
	secret, secretHash, err := newLookupSecret()
	if err != nil {
		return SubmissionResult{}, err
	}
	baseJSON, err := optionalSnapshotJSON(action, base)
	if err != nil {
		return SubmissionResult{}, err
	}
	proposedJSON, err := encodeSnapshot(proposed)
	if err != nil {
		return SubmissionResult{}, err
	}
	row, err := service.repository.queries.CreateSiteAudit(ctx, dbgen.CreateSiteAuditParams{
		LookupSecretHash: secretHash, Action: string(action), SiteID: targetID,
		BaseRevision: baseRevisionPointer(action, base), BaseSnapshot: baseJSON,
		ProposedSnapshot: proposedJSON, RequestReason: normalized.Reason,
		SubmitterName: stringPointer(normalized.Contact.Name), SubmitterEmail: stringPointer(normalized.Contact.Email),
		NotifyByEmail: normalized.Contact.NotifyByEmail,
	})
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && (databaseError.ConstraintName == "site_audits_pending_site_unique_idx" || databaseError.ConstraintName == "site_audits_pending_create_host_unique_idx") {
			return SubmissionResult{}, newServiceError("submission_pending", http.StatusConflict, "the site already has a pending submission")
		}
		return SubmissionResult{}, fmt.Errorf("create site audit: %w", err)
	}
	auditID, err := uuidString(row.ID)
	if err != nil {
		return SubmissionResult{}, err
	}
	return SubmissionResult{AuditID: auditID, LookupToken: secret, Action: action, Status: StatusPending, ShortID: targetShortID}, nil
}

func (service *Service) Query(ctx context.Context, lookupToken string) (PublicAuditResult, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(lookupToken))
	if err != nil || len(decoded) != 32 {
		return PublicAuditResult{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit lookup credential was not found")
	}
	digest := sha256.Sum256(decoded)
	row, err := service.repository.queries.GetSiteAuditByLookupHash(ctx, digest[:])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicAuditResult{}, newServiceError("audit_not_found", http.StatusNotFound, "the audit lookup credential was not found")
		}
		return PublicAuditResult{}, fmt.Errorf("query site audit by lookup credential: %w", err)
	}
	audit, err := auditFromRow(row)
	if err != nil {
		return PublicAuditResult{}, err
	}
	return PublicAuditResult{Action: audit.Action, Status: audit.Status, ShortID: publicAuditShortID(audit), ReviewerComment: audit.ReviewerComment, ReviewedAt: audit.ReviewedAt, CreatedAt: audit.CreatedAt}, nil
}

func proposedForAction(action Action, input SubmissionInput, base Snapshot) (Snapshot, error) {
	switch action {
	case ActionCreate, ActionUpdate:
		return BuildProposedSnapshot(input.Site, base)
	case ActionDelete:
		if base.Visibility == "REMOVED" {
			return Snapshot{}, newServiceError("site_already_removed", http.StatusConflict, "the target site is already removed")
		}
		base.Visibility = "REMOVED"
		base.VisibilityReason = input.Reason
		return base, nil
	case ActionRestore:
		if base.Visibility != "REMOVED" {
			return Snapshot{}, newServiceError("site_not_removed", http.StatusConflict, "only a removed site can be restored")
		}
		base.Visibility = "VISIBLE"
		base.VisibilityReason = ""
		return base, nil
	default:
		return Snapshot{}, fmt.Errorf("%w: unsupported action", ErrInvalidSubmission)
	}
}

func MergeRequestedSnapshot(base, proposed, current Snapshot) (Snapshot, []DiffItem) {
	merged := current
	conflicts := make([]DiffItem, 0)
	mergeString := func(field string, baseValue, proposedValue, currentValue string, destination *string) {
		if baseValue == proposedValue {
			return
		}
		if currentValue != baseValue && currentValue != proposedValue {
			conflicts = append(conflicts, DiffItem{Field: field, Before: currentValue, After: proposedValue})
			return
		}
		*destination = proposedValue
	}
	mergeString("name", base.Name, proposed.Name, current.Name, &merged.Name)
	mergeString("scheme", base.Scheme, proposed.Scheme, current.Scheme, &merged.Scheme)
	mergeString("normalized_host", base.NormalizedHost, proposed.NormalizedHost, current.NormalizedHost, &merged.NormalizedHost)
	mergeString("base_path", base.BasePath, proposed.BasePath, current.BasePath, &merged.BasePath)
	mergeString("summary", base.Summary, proposed.Summary, current.Summary, &merged.Summary)
	mergeString("access_scope", base.AccessScope, proposed.AccessScope, current.AccessScope, &merged.AccessScope)
	mergeString("visibility", base.Visibility, proposed.Visibility, current.Visibility, &merged.Visibility)
	mergeString("visibility_reason", base.VisibilityReason, proposed.VisibilityReason, current.VisibilityReason, &merged.VisibilityReason)
	mergeSlice("feeds", base.Feeds, proposed.Feeds, current.Feeds, &merged.Feeds, &conflicts)
	mergeSlice("resources", base.Resources, proposed.Resources, current.Resources, &merged.Resources, &conflicts)
	mergeSlice("tags", base.Tags, proposed.Tags, current.Tags, &merged.Tags, &conflicts)
	mergeSlice("components", base.Components, proposed.Components, current.Components, &merged.Components, &conflicts)
	mergeSlice("program_dependencies", base.ProgramDependencies, proposed.ProgramDependencies, current.ProgramDependencies, &merged.ProgramDependencies, &conflicts)
	return merged, conflicts
}

func mergeSlice[T any](field string, base, proposed, current []T, destination *[]T, conflicts *[]DiffItem) {
	if reflect.DeepEqual(base, proposed) {
		return
	}
	if !reflect.DeepEqual(current, base) && !reflect.DeepEqual(current, proposed) {
		*conflicts = append(*conflicts, DiffItem{Field: field})
		return
	}
	*destination = proposed
}

func optionalSnapshotJSON(action Action, snapshot Snapshot) ([]byte, error) {
	if action == ActionCreate {
		return nil, nil
	}
	return encodeSnapshot(snapshot)
}

func baseRevisionPointer(action Action, snapshot Snapshot) *int64 {
	if action == ActionCreate {
		return nil
	}
	value := snapshot.Revision
	return &value
}

func newLookupSecret() (string, []byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("generate audit lookup credential: %w", err)
	}
	digest := sha256.Sum256(secret)
	return base64.RawURLEncoding.EncodeToString(secret), digest[:], nil
}

type ServiceError struct {
	Code       string
	StatusCode int
	Detail     string
}

func (err *ServiceError) Error() string { return err.Code }

func newServiceError(code string, status int, detail string) *ServiceError {
	return &ServiceError{Code: code, StatusCode: status, Detail: detail}
}

func canReview(user auth.User) bool {
	return user.Role == auth.RoleSysAdmin || user.Role == auth.RoleAdmin && slicesContains(user.Permissions, auth.PermissionSiteAuditReview)
}

func canManageTaxonomy(user auth.User) bool {
	return user.Role == auth.RoleSysAdmin || user.Role == auth.RoleAdmin && slicesContains(user.Permissions, auth.PermissionTaxonomyManage)
}

func slicesContains(values []auth.Permission, expected auth.Permission) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (service *Service) CurrentReviewer(ctx context.Context, request *http.Request) (auth.User, error) {
	user, err := service.auth.Current(ctx, request)
	if err != nil {
		return auth.User{}, err
	}
	if !canReview(user) {
		return auth.User{}, newServiceError("forbidden", http.StatusForbidden, "site audit review permission is required")
	}
	return user, nil
}
