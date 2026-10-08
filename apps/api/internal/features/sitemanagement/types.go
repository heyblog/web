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
	DNS    Method = "DNS_TXT"
	Meta   Method = "META"
	File   Method = "FILE"
	Manual Method = "MANUAL"
)

type Claim struct {
	ID           string     `json:"id"`
	SiteID       string     `json:"-"`
	ShortID      string     `json:"short_id"`
	UserID       string     `json:"user_id"`
	Address      string     `json:"address"`
	Method       Method     `json:"method"`
	Status       string     `json:"status"`
	TokenHash    string     `json:"-"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Evidence     *string    `json:"evidence,omitempty"`
	EvidenceURL  *string    `json:"evidence_url,omitempty"`
	ReviewReason *string    `json:"review_reason,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
type Ownership struct {
	ID       string `json:"id"`
	SiteID   string `json:"-"`
	UserID   string `json:"user_id"`
	Address  string `json:"address"`
	Revision int64  `json:"revision"`
}
type Site struct{ ID, ShortID, Address, Visibility string }
type SiteClaimCreateInput struct {
	ShortID     string `json:"short_id" minLength:"9" maxLength:"9"`
	Method      Method `json:"method" enum:"DNS_TXT,META,FILE,MANUAL"`
	Address     string `json:"address,omitempty" maxLength:"2048"`
	Evidence    string `json:"evidence,omitempty" maxLength:"4000"`
	EvidenceURL string `json:"evidence_url,omitempty" maxLength:"2048"`
}
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
	UserID   string `json:"user_id,omitempty"`
	Reason   string `json:"reason" minLength:"1" maxLength:"2000"`
	Evidence string `json:"evidence" minLength:"1" maxLength:"4000"`
}
type Decision struct {
	Claim                   Claim
	Status, ActorID, Reason string
	Now                     time.Time
}
type StoredClaim struct {
	Claim  Claim
	UserID string
}
type Store interface {
	Site(context.Context, string) (Site, error)
	Owner(context.Context, string) (Ownership, error)
	Create(context.Context, StoredClaim) (Claim, error)
	Claim(context.Context, string) (Claim, error)
	List(context.Context, string, int32, int32) ([]Claim, error)
	Decide(context.Context, Decision) (Claim, error)
	ChangeOwner(context.Context, Site, OwnershipInput, string) error
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
