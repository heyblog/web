package siteaudit

import (
	"encoding/json"
	"time"

	"heyblog-api/internal/domain/site"
)

type AnnotationSite struct {
	ShortID        string          `json:"short_id"`
	Name           string          `json:"name"`
	URL            string          `json:"url"`
	BlogStatus     site.BlogStatus `json:"blog_status"`
	Revision       int64           `json:"revision"`
	UpdatedAt      time.Time       `json:"updated_at"`
	PendingAuditID string          `json:"pending_audit_id,omitempty"`
}

type AnnotationSelection struct {
	ID   string `json:"id" format:"uuid"`
	Role string `json:"role"`
}

type AnnotationTag struct {
	AnnotationSelection
	Name  string `json:"name"`
	Level int    `json:"level,omitempty"`
}

type AnnotationComponent struct {
	AnnotationSelection
	Name string `json:"name"`
}

type AnnotationFeed struct {
	FeedInput
	ID        string `json:"id,omitempty" format:"uuid"`
	IsEnabled bool   `json:"is_enabled"`
}

type AnnotationFriendInput struct {
	TargetURL  string `json:"target_url" maxLength:"2048"`
	LinkStatus string `json:"link_status" enum:"ACTIVE,INACTIVE"`
}

type AnnotationOutgoing struct {
	AnnotationFriendInput
	TargetHost string `json:"target_host"`
}

type AnnotationIncoming struct {
	SourceShortID string `json:"source_short_id"`
	SourceName    string `json:"source_name"`
	SourceURL     string `json:"source_url"`
	LinkStatus    string `json:"link_status"`
}

type AnnotationDetail struct {
	AnnotationSite
	Summary       string                `json:"summary"`
	AccessScope   string                `json:"access_scope"`
	TagCascadeID  string                `json:"tag_cascade_id"`
	Visibility    string                `json:"visibility"`
	Tags          []AnnotationTag       `json:"tags"`
	Components    []AnnotationComponent `json:"components"`
	Feeds         []AnnotationFeed      `json:"feeds"`
	Resources     []ResourceSnapshot    `json:"resources"`
	Outgoing      []AnnotationOutgoing  `json:"outgoing"`
	Incoming      []AnnotationIncoming  `json:"incoming"`
	FriendVersion string                `json:"friend_version"`
}

type AnnotationPatch struct {
	ExpectedRevision int64                   `json:"expected_revision" minimum:"1"`
	FriendVersion    string                  `json:"friend_version" minLength:"64" maxLength:"64"`
	Reason           string                  `json:"reason,omitempty" maxLength:"2000"`
	Name             *string                 `json:"name,omitempty" maxLength:"160"`
	URL              *string                 `json:"url,omitempty" maxLength:"2048"`
	Summary          *string                 `json:"summary,omitempty" maxLength:"2000"`
	AccessScope      *string                 `json:"access_scope,omitempty" enum:"ALL,CN_ONLY,GLOBAL_ONLY"`
	BlogStatus       *site.BlogStatus        `json:"blog_status,omitempty" enum:"UNREVIEWED,BLOG,NON_BLOG"`
	TagCascadeID     *string                 `json:"tag_cascade_id,omitempty" format:"uuid"`
	Tags             []AnnotationSelection   `json:"tags,omitempty" maxItems:"40"`
	Components       []AnnotationSelection   `json:"components,omitempty" maxItems:"50"`
	Feeds            []AnnotationFeed        `json:"feeds,omitempty" maxItems:"8"`
	Resources        []ResourceInput         `json:"resources,omitempty" maxItems:"2"`
	Outgoing         []AnnotationFriendInput `json:"outgoing,omitempty"`
}

type AnnotationHistory struct {
	ID        string          `json:"id"`
	ActorName string          `json:"actor_name"`
	CreatedAt time.Time       `json:"created_at"`
	Revision  int64           `json:"revision"`
	AuditID   string          `json:"site_audit_id,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	Source    string          `json:"source"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
}

type annotationPage[T any] struct {
	Items    []T   `json:"items"`
	Page     int32 `json:"page"`
	PageSize int32 `json:"page_size"`
	Total    int64 `json:"total"`
}
