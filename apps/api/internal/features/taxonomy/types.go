package taxonomy

type Tag struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	Description  string   `json:"description"`
	Enabled      bool     `json:"is_enabled"`
	SiteCount    int      `json:"site_count"`
	ArticleCount int      `json:"article_count"`
	Roles        []string `json:"roles"`
	SystemKey    string   `json:"-"`
}

type Cascade struct {
	ID           string `json:"id"`
	Scope        string `json:"scope"`
	PrimaryID    string `json:"level1_tag_id"`
	SecondaryID  string `json:"level2_tag_id"`
	Key          string `json:"taxonomy_key"`
	Enabled      bool   `json:"is_enabled"`
	MergedIntoID string `json:"merged_into_id"`
}

type Catalog struct {
	Tags     []Tag     `json:"tags"`
	Cascades []Cascade `json:"cascades"`
	Revision string    `json:"revision"`
}

type CreateInput struct {
	Name             string `json:"name" minLength:"1" maxLength:"120"`
	Slug             string `json:"slug" minLength:"1" maxLength:"160" pattern:"^[a-z0-9]+(-[a-z0-9]+)*$"`
	Description      string `json:"description" maxLength:"2000"`
	ExpectedRevision string `json:"expected_revision" minLength:"1"`
}

type UpdateInput struct {
	Name             string `json:"name" minLength:"1" maxLength:"120"`
	Slug             string `json:"slug" minLength:"1" maxLength:"160" pattern:"^[a-z0-9]+(-[a-z0-9]+)*$"`
	Description      string `json:"description" maxLength:"2000"`
	Enabled          bool   `json:"is_enabled"`
	ExpectedRevision string `json:"expected_revision" minLength:"1"`
}

type DeleteInput struct {
	ExpectedRevision string `json:"expected_revision" minLength:"1"`
}
type ChangeInput struct {
	Kind             string `json:"kind" enum:"merge,path_update,path_merge"`
	SourceID         string `json:"source_id"`
	PrimaryID        string `json:"primary_id,omitempty"`
	SecondaryID      string `json:"secondary_id,omitempty"`
	Enabled          bool   `json:"is_enabled,omitempty"`
	TargetID         string `json:"target_id,omitempty"`
	ExpectedRevision string `json:"expected_revision" minLength:"1"`
	Fingerprint      string `json:"fingerprint,omitempty"`
}

type PathImpact struct {
	CascadeID string `json:"cascade_id"`
	Scope     string `json:"scope"`
	Label     string `json:"label"`
}

type Preview struct {
	Revision          string       `json:"revision"`
	Fingerprint       string       `json:"fingerprint"`
	SiteCount         int          `json:"site_count"`
	ArticleCount      int          `json:"article_count"`
	RemovedDuplicates int          `json:"removed_duplicates"`
	Blockers          []string     `json:"blockers"`
	Paths             []PathImpact `json:"paths"`
}

type assignment struct {
	TagID     string
	Role      string
	Source    string
	Position  int16
	Note      *string
	CreatedAt string
}

type object struct {
	ID        string
	Scope     string
	CascadeID string
	Version   string
	Tags      []assignment
}

type graph struct {
	Catalog
	Objects []object
}

type changePlan struct {
	Preview
	Graph    graph
	Affected map[string]bool
	Merged   map[string]string
}

type CreateCascadeInput struct {
	Scope            string `json:"scope" enum:"SITE,ARTICLE"`
	PrimaryID        string `json:"primary_id"`
	SecondaryID      string `json:"secondary_id"`
	Key              string `json:"taxonomy_key" maxLength:"160" pattern:"^[a-z0-9]+(-[a-z0-9]+)*/[a-z0-9]+(-[a-z0-9]+)*$"`
	ExpectedRevision string `json:"expected_revision" minLength:"1"`
}
