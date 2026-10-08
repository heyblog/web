package databasebackup

import (
	"errors"
	"time"
)

const (
	Path            = "/management/database-backup"
	FileLimit int64 = 512 << 20
	BodyLimit       = FileLimit + 1<<20
	Timeout         = 30 * time.Minute
	Format          = "heyblog.database-backup"
)

var (
	ErrInvalid  = errors.New("invalid database backup")
	ErrTarget   = errors.New("database restore target is not initialized")
	ErrTooLarge = errors.New("database backup exceeds size limit")
)

// Table order follows foreign-key dependencies. Self-references are deferred by
// the controlled restore entry point and checked before commit.
var tableNames = []string{
	"identity.users", "identity.oauth_identities", "identity.email_verification_codes",
	"identity.password_reset_tokens", "identity.user_management_permissions",
	"identity.api_clients", "identity.api_client_scopes", "identity.api_keys",
	"directory.tags", "directory.tag_labels", "directory.tag_cascades",
	"directory.tag_identity_aliases", "directory.tag_slug_aliases", "directory.tag_assignment_archive",
	"directory.software_components", "directory.software_component_dependencies", "directory.site_sources",
	"directory.sites", "directory.site_feeds", "directory.site_icons", "directory.site_resources",
	"directory.site_software_components", "directory.site_origins", "directory.site_tags",
	"directory.site_claims", "directory.site_ownerships", "directory.site_ownership_events",
	"directory.site_audits", "directory.owner_friend_link_requests", "directory.slug_generation_cache", "directory.slug_generation_jobs",
	"directory.site_metrics", "directory.site_metric_events",
	"content.articles", "content.article_tags", "content.announcements", "content.announcement_revisions",
	"content.system_ai_settings",
}

type Header struct {
	Format                string    `json:"format"`
	Version               int       `json:"version"`
	SchemaVersion         int       `json:"schema_version"`
	GeneratedAt           time.Time `json:"generated_at"`
	SourceRevision        string    `json:"source_revision"`
	ExcludedSystemAdminID string    `json:"excluded_system_admin_id"`
}

type Dataset struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type Manifest struct {
	Dataset
	SHA256 string `json:"sha256"`
}

type GraphCounts struct {
	Vertices int64 `json:"vertices"`
	Edges    int64 `json:"edges"`
}

type Issue struct {
	Code    string `json:"code"`
	Dataset string `json:"dataset,omitempty"`
}

type Inspection struct {
	SHA256                string      `json:"sha256"`
	Version               int         `json:"version"`
	SchemaVersion         int         `json:"schema_version"`
	GeneratedAt           time.Time   `json:"generated_at"`
	ExcludedSystemAdminID string      `json:"excluded_system_admin_id"`
	RetainedSystemAdminID string      `json:"retained_system_admin_id"`
	Datasets              []Dataset   `json:"datasets"`
	Graph                 GraphCounts `json:"graph"`
	TargetReady           bool        `json:"target_ready"`
	Issues                []Issue     `json:"issues"`
	CanRestore            bool        `json:"can_restore"`
}

type AdminMapping struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}

type Restoration struct {
	Status       string       `json:"status"`
	SHA256       string       `json:"sha256"`
	Datasets     []Dataset    `json:"datasets"`
	Graph        GraphCounts  `json:"graph"`
	AdminMapping AdminMapping `json:"admin_mapping"`
}
