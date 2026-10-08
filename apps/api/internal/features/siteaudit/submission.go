package siteaudit

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"heyblog-api/internal/domain/site"
)

func (service *Service) Submit(
	ctx context.Context,
	action Action,
	targetShortID string,
	input SubmissionInput,
) (SubmissionResult, error) {
	return service.submit(ctx, action, targetShortID, input, submissionProvenance{Channel: "ANONYMOUS"})
}

func (service *Service) submit(ctx context.Context, action Action, targetShortID string, input SubmissionInput, provenance submissionProvenance) (SubmissionResult, error) {
	normalized, err := NormalizeSubmission(action, input)
	if err != nil {
		return SubmissionResult{}, err
	}
	base := Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"}
	if action != ActionCreate {
		if err := site.ValidateShortID(targetShortID); err != nil {
			return SubmissionResult{}, fmt.Errorf("%w: target site short ID is invalid", ErrInvalidSubmission)
		}
		var err error
		base, err = service.repository.SubmissionSite(ctx, targetShortID)
		if err != nil {
			return SubmissionResult{}, err
		}
	}
	proposed, err := proposedForAction(action, normalized, base)
	if err != nil {
		return SubmissionResult{}, err
	}
	if err := service.ensureCreateAddressAvailable(ctx, action, proposed.NormalizedHost); err != nil {
		return SubmissionResult{}, err
	}
	if action == ActionCreate || action == ActionUpdate {
		proposed, err = service.repository.PrepareSubmission(ctx, proposed, base)
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
	auditID, err := service.repository.CreateSubmission(ctx, submissionRecord{
		Action: action, Base: base, Proposed: proposed, Input: normalized, LookupHash: secretHash, Provenance: provenance,
	})
	if err != nil {
		return SubmissionResult{}, err
	}
	if provenance.UserID != "" {
		secret = ""
	}
	return SubmissionResult{AuditID: auditID, LookupToken: secret, Action: action, Status: StatusPending, ShortID: targetShortID}, nil
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

func newLookupSecret() (string, []byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("generate audit lookup credential: %w", err)
	}
	digest := sha256.Sum256(secret)
	return base64.RawURLEncoding.EncodeToString(secret), digest[:], nil
}
