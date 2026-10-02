package dataimport

type BlogBundle struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	GeneratedAt string          `json:"generated_at"`
	Inputs      []InputMetadata `json:"inputs"`
	Count       int             `json:"count"`
	Blogs       []LegacyBlog    `json:"blogs"`
}

type LegacyBlog struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	URL              string              `json:"url"`
	Summary          string              `json:"summary"`
	Feeds            []LegacyFeed        `json:"feeds"`
	Sitemap          *string             `json:"sitemap"`
	LinkPage         *string             `json:"link_page"`
	JoinedAt         string              `json:"joined_at"`
	UpdatedAt        string              `json:"updated_at"`
	AccessScope      string              `json:"access_scope"`
	Visibility       string              `json:"visibility"`
	VisibilityReason *string             `json:"visibility_reason"`
	Origins          []LegacyOrigin      `json:"origins"`
	MainTag          *LegacyTag          `json:"main_tag"`
	SubTags          []LegacyTag         `json:"sub_tags"`
	Architecture     *LegacyArchitecture `json:"architecture"`
}

type LegacyFeed struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Format    string `json:"format"`
}

type LegacyOrigin struct {
	SourceKey         string         `json:"source_key"`
	ExternalReference string         `json:"external_reference"`
	FirstDiscoveredAt string         `json:"first_discovered_at"`
	Metadata          OriginMetadata `json:"metadata"`
}

type OriginMetadata struct {
	InputKinds         []string `json:"input_kinds"`
	ExternalReferences []string `json:"external_references"`
}

type LegacyTag struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	MachineKey  *string `json:"machine_key"`
	Description *string `json:"description"`
	IsEnabled   bool    `json:"is_enabled"`
}

type LegacyArchitecture struct {
	Program          LegacyProgram `json:"program"`
	TechnologyStacks []LegacyStack `json:"technology_stacks"`
}

type LegacyProgram struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"name_normalized"`
	IsOpenSource   bool    `json:"is_open_source"`
	WebsiteURL     *string `json:"website_url"`
	RepositoryURL  *string `json:"repo_url"`
	IsEnabled      bool    `json:"is_enabled"`
}

type LegacyStack struct {
	ID             string         `json:"id"`
	Category       string         `json:"category"`
	Name           string         `json:"name"`
	NormalizedName string         `json:"name_normalized"`
	Catalog        *LegacyCatalog `json:"catalog"`
}

type LegacyCatalog struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"name_normalized"`
	TechnologyType string  `json:"technology_type"`
	Description    *string `json:"description"`
	OfficialURL    *string `json:"official_url"`
	IsEnabled      bool    `json:"is_enabled"`
}
