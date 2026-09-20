package apikey

import "time"

type Audience string

const (
	AudienceInternal Audience = "INTERNAL"
	AudienceExternal Audience = "EXTERNAL"
)

type Scope string

const (
	ScopeDataImportWrite Scope = "data_import.write"
	ScopeExampleCall     Scope = "example.call"
)

type AccessPolicy struct {
	Audiences []Audience
	Scope     Scope
}

type Client struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Audience    Audience   `json:"audience"`
	Scopes      []Scope    `json:"scopes"`
	DisabledAt  *time.Time `json:"disabled_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Keys        []Key      `json:"keys"`
}

type Key struct {
	ID          string     `json:"id"`
	Prefix      string     `json:"prefix"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	RotatedFrom *string    `json:"rotated_from_id"`
}

type Credential struct {
	Client Client `json:"client"`
	Key    Key    `json:"key"`
	Token  string `json:"token"`
}

type Principal struct {
	ClientID string
	KeyID    string
	Audience Audience
	Scopes   []Scope
}

type StoredCredential struct {
	Principal
	SecretHash       []byte
	ExpiresAt        *time.Time
	RevokedAt        *time.Time
	ClientDisabledAt *time.Time
	LastUsedAt       *time.Time
}

type CreateClientRequest struct {
	Name         string
	Description  string
	Audience     Audience
	Scopes       []Scope
	ExpiresAt    *time.Time
	NeverExpires bool
	CreatedBy    string
}

type CreateClientRecord struct {
	CreateClientRequest
	PublicID   string
	Prefix     string
	SecretHash []byte
}

type RotateRequest struct {
	ClientID string
	Overlap  time.Duration
	ActorID  string
}

type RotateRecord struct {
	RotateRequest
	Now        time.Time
	PublicID   string
	Prefix     string
	SecretHash []byte
}

type IssueKeyRequest struct {
	ClientID     string
	ExpiresAt    *time.Time
	NeverExpires bool
	ActorID      string
}

type IssueKeyRecord struct {
	IssueKeyRequest
	Now        time.Time
	PublicID   string
	Prefix     string
	SecretHash []byte
}

type UpdateClientRecord struct {
	ID          string
	Name        string
	Description string
	Scopes      []Scope
	Enabled     bool
	ActorID     string
}

type RevokeKeyRecord struct {
	KeyID   string
	ActorID string
	Now     time.Time
}
