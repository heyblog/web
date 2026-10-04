package announcement

import "time"

type Input struct {
	Kind              string     `json:"kind" enum:"MAIN,BANNER"`
	Title             string     `json:"title" minLength:"1" maxLength:"256"`
	BodyMarkdown      *string    `json:"bodyMarkdown"`
	Priority          int32      `json:"priority"`
	ActionType        string     `json:"actionType" enum:"NONE,INTERNAL,EXTERNAL"`
	ActionLabel       *string    `json:"actionLabel"`
	ActionPath        *string    `json:"actionPath"`
	ActionExternalURL *string    `json:"actionExternalUrl"`
	StartsAt          *time.Time `json:"startsAt"`
	EndsAt            *time.Time `json:"endsAt"`
}

type EditInput struct {
	Input
	RowVersion string `json:"rowVersion" pattern:"^[1-9][0-9]*$"`
}

type VersionInput struct {
	RowVersion string `json:"rowVersion" pattern:"^[1-9][0-9]*$"`
}
type PublishInput struct {
	VersionInput
	StartsAt *time.Time `json:"startsAt" required:"false"`
	EndsAt   *time.Time `json:"endsAt" required:"false"`
}

type ManagedAnnouncement struct {
	ID string `json:"id"`
	Input
	Status          string     `json:"status"`
	EffectiveStatus string     `json:"effectiveStatus"`
	RowVersion      string     `json:"rowVersion"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	PublishedAt     *time.Time `json:"publishedAt"`
	ArchivedAt      *time.Time `json:"archivedAt"`
	CreatedBy       *string    `json:"createdBy"`
	UpdatedBy       *string    `json:"updatedBy"`
	PublishedBy     *string    `json:"publishedBy"`
	ArchivedBy      *string    `json:"archivedBy"`
}

type Revision struct {
	Input
	Revision    string    `json:"revision"`
	PublishedAt time.Time `json:"publishedAt"`
	PublishedBy *string   `json:"publishedBy"`
	ChangedAt   time.Time `json:"changedAt"`
	ChangedBy   *string   `json:"changedBy"`
}

type List struct {
	Announcements []ManagedAnnouncement `json:"announcements"`
	Total         int64                 `json:"total"`
	Page          int32                 `json:"page"`
	PageSize      int32                 `json:"pageSize"`
}

type ListQuery struct {
	Kind     string `query:"kind" enum:"MAIN,BANNER"`
	Status   string `query:"status" enum:"DRAFT,PUBLISHED,ARCHIVED"`
	Page     int32  `query:"page" default:"1" minimum:"1" maximum:"21474836"`
	PageSize int32  `query:"pageSize" default:"20" minimum:"1" maximum:"100"`
}
