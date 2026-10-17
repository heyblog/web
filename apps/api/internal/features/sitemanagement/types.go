package sitemanagement

import (
	"context"
	"errors"
	"net/http"
	"time"

	"heyblog-api/internal/features/auth"
)

type Method string

const (
	DNS       Method = "DNS_TXT"
	Meta      Method = "META"
	File      Method = "FILE"
	Manual    Method = "MANUAL"
	BlogsClub Method = "BLOGSCLUB"
)

type Claim struct {
	ID             string            `json:"id"`
	SiteID         string            `json:"-"`
	ShortID        string            `json:"short_id"`
	SiteName       string            `json:"site_name"`
	CurrentAddress string            `json:"current_address"`
	Visibility     string            `json:"-"`
	OwnerUserID    string            `json:"-"`
	OwnerAddress   string            `json:"-"`
	Applicant      *Profile          `json:"applicant,omitempty"`
	OwnershipState string            `json:"ownership_state"`
	CanCheck       bool              `json:"can_check"`
	CanRegenerate  bool              `json:"can_regenerate"`
	CanCancel      bool              `json:"can_cancel"`
	CanManage      bool              `json:"can_manage"`
	VerifiedAt     *time.Time        `json:"verified_at,omitempty"`
	ConsumedAt     *time.Time        `json:"consumed_at,omitempty"`
	UserID         string            `json:"user_id"`
	Address        string            `json:"address"`
	Method         Method            `json:"method"`
	Status         string            `json:"status"`
	TokenHash      string            `json:"-"`
	ExpiresAt      *time.Time        `json:"expires_at,omitempty"`
	Evidence       *string           `json:"evidence,omitempty"`
	EvidenceURL    *string           `json:"evidence_url,omitempty"`
	ReviewReason   *string           `json:"review_reason,omitempty"`
	ReviewedAt     *time.Time        `json:"reviewed_at,omitempty"`
	AttachmentIDs  []string          `json:"attachment_ids,omitempty"`
	Attachments    []ClaimAttachment `json:"attachments,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}
type Ownership struct {
	ID        string  `json:"id"`
	SiteID    string  `json:"-"`
	UserID    string  `json:"user_id"`
	Address   string  `json:"address"`
	Revision  int64   `json:"revision"`
	Owner     Profile `json:"owner"`
	IsCurrent bool    `json:"is_current"`
}
type Profile struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}
type Site struct {
	ID         string `json:"-"`
	ShortID    string `json:"short_id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Visibility string `json:"visibility"`
}
type ClaimContext struct {
	Site             Site    `json:"site"`
	OwnershipState   string  `json:"ownership_state"`
	CanClaim         bool    `json:"can_claim"`
	CanManage        bool    `json:"can_manage"`
	CanVerifyAddress bool    `json:"can_verify_address"`
	PendingClaims    []Claim `json:"pending_claims"`
}
type OwnershipEvent struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id"`
	ActorID   *string   `json:"actor_id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Evidence  *string   `json:"evidence"`
	CreatedAt time.Time `json:"created_at"`
}
type OwnershipDetail struct {
	Site      Site             `json:"site"`
	Current   *Ownership       `json:"current"`
	History   []OwnershipEvent `json:"history"`
	CanRevoke bool             `json:"can_revoke"`
}
type ClaimFilter struct {
	UserID, SiteID, Status, Method string
	Limit, Offset                  int32
}
type ClaimList struct {
	Items   []Claim `json:"items"`
	HasMore bool    `json:"has_more"`
}

type AttachmentInitializeInput struct {
	ShortID     string `json:"short_id" minLength:"9" maxLength:"9"`
	Address     string `json:"address,omitempty" maxLength:"2048"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size" minimum:"1" maximum:"10485760"`
}

type AttachmentCompleteInput struct {
	Size        int64  `json:"size" minimum:"1" maximum:"10485760"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256" pattern:"^[0-9a-f]{64}$"`
}

type AttachmentUploadResult struct {
	Attachment    ClaimAttachment   `json:"attachment"`
	UploadURL     string            `json:"upload_url"`
	UploadHeaders map[string]string `json:"upload_headers"`
	ExpiresAt     time.Time         `json:"expires_at"`
	MaxSize       int64             `json:"max_size"`
	ContentType   AttachmentKind    `json:"content_type"`
}

type SiteClaimCreateInput struct {
	ShortID       string   `json:"short_id" minLength:"9" maxLength:"9"`
	Method        Method   `json:"method" enum:"DNS_TXT,META,FILE,MANUAL,BLOGSCLUB"`
	Address       string   `json:"address,omitempty" maxLength:"2048"`
	Evidence      string   `json:"evidence,omitempty" maxLength:"4000"`
	EvidenceURL   string   `json:"evidence_url,omitempty" maxLength:"2048"`
	AttachmentIDs []string `json:"attachment_ids,omitempty" maxItems:"3" uniqueItems:"true"`
}

// StoredClaim is the transaction input used by the repository. AttachmentIDs
// are bound to the claim in the same transaction as the claim insert.
type Instructions struct {
	DNSName     string `json:"dns_name"`
	DNSValue    string `json:"dns_value"`
	Meta        string `json:"meta"`
	FileURL     string `json:"file_url"`
	FileContent string `json:"file_content"`
}
type CreateResult struct {
	Claim        Claim         `json:"claim"`
	Token        string        `json:"token,omitempty"`
	Instructions *Instructions `json:"instructions,omitempty"`
}
type SiteClaimReviewInput struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason" minLength:"1" maxLength:"2000"`
}
type OwnershipInput struct {
	OwnershipID string `json:"ownership_id" format:"uuid"`
	Revision    int64  `json:"revision" minimum:"1"`
	Reason      string `json:"reason" minLength:"1" maxLength:"2000"`
	Evidence    string `json:"evidence" minLength:"1" maxLength:"4000"`
}
type Decision struct {
	Claim                   Claim
	Status, ActorID, Reason string
	Now                     time.Time
	OAuthActor              *auth.User
	OAuthSubject            string
}
type StoredClaim struct {
	Claim         Claim
	UserID        string
	AttachmentIDs []string
}
type AttachmentStore interface {
	InitializeAttachment(context.Context, string, Site, AttachmentUploadInput) (ClaimAttachment, EvidenceUpload, error)
	CompleteAttachment(context.Context, string, string, AttachmentUploadInput) (ClaimAttachment, error)
	DeleteAttachment(context.Context, string, string) error
	Settings() AttachmentSettings
	DownloadAttachment(context.Context, string, string, bool) (EvidenceDownload, error)
}
type Store interface {
	Site(context.Context, string) (Site, error)
	Owner(context.Context, string) (Ownership, error)
	Create(context.Context, StoredClaim) (Claim, error)
	Claim(context.Context, string) (Claim, error)
	List(context.Context, ClaimFilter) ([]Claim, error)
	Regenerate(context.Context, Claim, string, time.Time) (Claim, error)
	History(context.Context, string) ([]OwnershipEvent, error)
	Decide(context.Context, Decision) (Claim, error)
	Revoke(context.Context, Site, OwnershipInput, string) error
}
type Authentication interface {
	Current(context.Context, *http.Request) (auth.User, error)
}
type Verifier interface {
	Verify(context.Context, Claim) error
}

var ErrNotFound = errors.New("site verification not found")

type ServiceError struct {
	Code, Message string
	Status        int
}

func (e *ServiceError) Error() string { return e.Message }
func failure(code, message string, status int) error {
	return &ServiceError{Code: code, Message: message, Status: status}
}
