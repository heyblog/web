package publicview

import "time"

type SiteCard struct {
	ShortID         string          `json:"shortId"`
	CustomID        *string         `json:"customId"`
	Name            string          `json:"name"`
	Summary         string          `json:"summary"`
	Host            string          `json:"host"`
	HomepageURL     string          `json:"homepageUrl"`
	AccessScope     string          `json:"accessScope"`
	DirectoryStatus DirectoryStatus `json:"directoryStatus"`
	JoinedAt        time.Time       `json:"joinedAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type SiteProfile struct {
	SiteCard
	IconHash       *string                    `json:"iconHash"`
	Classification *SiteProfileClassification `json:"classification"`
	TertiaryTags   []Topic                    `json:"tertiaryTags"`
	Warnings       []Warning                  `json:"warnings"`
	Feeds          []Feed                     `json:"feeds"`
	Resources      []Resource                 `json:"resources"`
	Technologies   []Technology               `json:"technologies"`
}

type SiteProfileClassification struct {
	Level1 Topic `json:"level1"`
	Level2 Topic `json:"level2"`
}

type Topic struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type Warning struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type Feed struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Format    string `json:"format"`
	IsDefault bool   `json:"isDefault"`
}

type Resource struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type Technology struct {
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	HomepageURL   *string `json:"homepageUrl"`
	RepositoryURL *string `json:"repositoryUrl"`
	IsOpenSource  bool    `json:"isOpenSource"`
}
