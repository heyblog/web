package siteaudit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"

	"heyblog-api/internal/auth"
)

func TestNewLookupSecretReturnsCredentialWhoseDigestMatchesStoredHash(t *testing.T) {
	t.Parallel()

	credential, storedHash, err := newLookupSecret()

	if err != nil {
		t.Fatalf("newLookupSecret() error = %v", err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(credential)
	if err != nil {
		t.Fatalf("decode credential: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("credential entropy bytes = %d, want 32", len(decoded))
	}
	digest := sha256.Sum256(decoded)
	if string(storedHash) != string(digest[:]) {
		t.Error("stored hash does not match the credential digest")
	}
}

func TestQueryReturnsNotFoundForMalformedCredentialBeforeRepositoryAccess(t *testing.T) {
	t.Parallel()

	_, err := (&Service{}).Query(context.Background(), "not-a-credential")

	var serviceError *ServiceError
	if !errors.As(err, &serviceError) {
		t.Fatalf("Query() error = %v, want ServiceError", err)
	}
	if serviceError.Code != "audit_not_found" || serviceError.StatusCode != http.StatusNotFound {
		t.Errorf("Query() error = %#v, want audit_not_found 404", serviceError)
	}
}

func TestPublicSiteSnapshotOmitsInternalUUID(t *testing.T) {
	t.Parallel()

	snapshot := publicSiteSnapshot(Snapshot{
		SiteID:  "550e8400-e29b-41d4-a716-446655440000",
		ShortID: "A1b2C3d4E",
	})

	if snapshot.SiteID != "" {
		t.Errorf("public snapshot SiteID = %q, want empty", snapshot.SiteID)
	}
	if snapshot.ShortID != "A1b2C3d4E" {
		t.Errorf("public snapshot ShortID = %q, want A1b2C3d4E", snapshot.ShortID)
	}
}

func TestPublicAuditShortIDPrefersApprovedFinalSnapshot(t *testing.T) {
	t.Parallel()

	shortID := publicAuditShortID(Audit{
		BaseSnapshot:     Snapshot{ShortID: "A1b2C3d4E"},
		ProposedSnapshot: Snapshot{ShortID: "A1b2C3d4E"},
		FinalSnapshot:    Snapshot{ShortID: "Z9y8X7w6V"},
	})

	if shortID != "Z9y8X7w6V" {
		t.Errorf("public audit ShortID = %q, want Z9y8X7w6V", shortID)
	}
}

func TestReviewPermissionsSeparateAuditReviewFromTaxonomyManagement(t *testing.T) {
	t.Parallel()

	reviewer := auth.User{Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionSiteAuditReview}}
	taxonomyManager := auth.User{Role: auth.RoleAdmin, Permissions: []auth.Permission{auth.PermissionTaxonomyManage}}

	if !canReview(reviewer) {
		t.Error("site audit reviewer should be allowed to review")
	}
	if canManageTaxonomy(reviewer) {
		t.Error("site audit reviewer should not implicitly manage taxonomy")
	}
	if canReview(taxonomyManager) {
		t.Error("taxonomy manager should not implicitly review site audits")
	}
}

func TestProposedDeleteUsesSubmittedReasonAsVisibilityReason(t *testing.T) {
	t.Parallel()

	proposed, err := proposedForAction(
		ActionDelete,
		SubmissionInput{Reason: "Site owner requested removal."},
		Snapshot{Visibility: "VISIBLE"},
	)

	if err != nil {
		t.Fatalf("proposedForAction() error = %v", err)
	}
	if proposed.VisibilityReason != "Site owner requested removal." {
		t.Errorf("VisibilityReason = %q, want submitted reason", proposed.VisibilityReason)
	}
}

func TestMergeRequestedSnapshotAppliesCreateAccessScope(t *testing.T) {
	t.Parallel()

	merged, conflicts := MergeRequestedSnapshot(
		Snapshot{},
		Snapshot{AccessScope: "ALL", Visibility: "VISIBLE"},
		Snapshot{},
	)

	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
	if merged.AccessScope != "ALL" || merged.Visibility != "VISIBLE" {
		t.Fatalf("merged directory state = (%q, %q), want (ALL, VISIBLE)", merged.AccessScope, merged.Visibility)
	}
}
