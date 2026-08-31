package dataimport

import (
	"encoding/json"
	"time"
)

type Plan struct {
	BlogSHA256     string
	GraphSHA256    string
	Sites          []SiteRow
	Feeds          []FeedRow
	Resources      []ResourceRow
	Tags           []TagRow
	SiteTags       []SiteTagRow
	Components     []ComponentRow
	Dependencies   []DependencyRow
	SiteComponents []SiteComponentRow
	Sources        []SourceRow
	Origins        []OriginRow
	FriendLinks    []FriendLinkRow
}

type SiteRow struct {
	ID               string
	ShortID          string
	Name             string
	Scheme           string
	NormalizedHost   string
	BasePath         string
	Summary          string
	AccessScope      string
	Visibility       string
	VisibilityReason string
	JoinedAt         time.Time
	UpdatedAt        time.Time
}

type FeedRow struct {
	SiteID       string
	Name         string
	LocationType string
	URLRef       string
	ExternalURL  string
	URLKey       string
	Format       string
	IsEnabled    bool
	IsDefault    bool
}

type ResourceRow struct {
	SiteID       string
	Kind         string
	LocationType string
	URLRef       string
	ExternalURL  string
	URLKey       string
}

type TagRow struct {
	ID             string
	Name           string
	NormalizedName string
	Slug           string
	Description    string
	IsEnabled      bool
}

type SiteTagRow struct {
	SiteID string
	TagID  string
	Role   string
	Note   string
}

type ComponentRow struct {
	ID             string
	Name           string
	NormalizedName string
	Description    string
	HomepageURL    string
	RepositoryURL  string
	IsOpenSource   bool
	IsEnabled      bool
}

type DependencyRow struct {
	ComponentID           string
	DependencyComponentID string
	Role                  string
}

type SiteComponentRow struct {
	SiteID       string
	ComponentID  string
	Role         string
	IdentifiedAt time.Time
}

type SourceRow struct {
	Key  string
	Name string
}

type OriginRow struct {
	SiteID            string
	SourceKey         string
	ExternalReference string
	FirstDiscoveredAt time.Time
	Metadata          json.RawMessage
}

type FriendLinkRow struct {
	SourceSiteID string
	TargetURL    string
	TargetHost   string
}

type Counts struct {
	Sites              int `json:"sites"`
	Feeds              int `json:"feeds"`
	Resources          int `json:"resources"`
	Tags               int `json:"tags"`
	SiteTags           int `json:"site_tags"`
	SoftwareComponents int `json:"software_components"`
	Dependencies       int `json:"dependencies"`
	SiteComponents     int `json:"site_components"`
	Sources            int `json:"sources"`
	Origins            int `json:"origins"`
	FriendLinks        int `json:"friend_links"`
}

func (plan Plan) Counts() Counts {
	return Counts{
		Sites: len(plan.Sites), Feeds: len(plan.Feeds), Resources: len(plan.Resources),
		Tags: len(plan.Tags), SiteTags: len(plan.SiteTags), SoftwareComponents: len(plan.Components),
		Dependencies: len(plan.Dependencies), SiteComponents: len(plan.SiteComponents),
		Sources: len(plan.Sources), Origins: len(plan.Origins), FriendLinks: len(plan.FriendLinks),
	}
}
