package publicview

import (
	"context"
	"errors"
	"time"

	"heyblog-api/internal/domain/content"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type Announcement struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	BodyMarkdown *string             `json:"bodyMarkdown"`
	StartsAt     time.Time           `json:"startsAt"`
	PublishedAt  time.Time           `json:"publishedAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
	EndsAt       *time.Time          `json:"endsAt"`
	Action       *AnnouncementAction `json:"action"`
}

type AnnouncementAction struct {
	Label    string `json:"label"`
	Href     string `json:"href"`
	External bool   `json:"external"`
}

func loadAnnouncements(ctx context.Context, queries AnnouncementQueries) ([]Announcement, error) {
	rows, err := queries.ListActiveMainAnnouncements(ctx)
	if err != nil {
		return nil, internalError(err, "load active announcements")
	}
	result := make([]Announcement, 0, len(rows))
	for _, row := range rows {
		if row.Kind != string(content.KindMain) || row.Status != string(content.StatusPublished) {
			return nil, internalError(errors.New("invalid active main announcement"), "map active announcements")
		}
		view, err := mapAnnouncement(row)
		if err != nil {
			return nil, err
		}
		result = append(result, *view)
	}
	return result, nil
}

func mapAnnouncement(row dbgen.ContentAnnouncement) (*Announcement, error) {
	if !row.StartsAt.Valid || !row.PublishedAt.Valid || !row.UpdatedAt.Valid {
		return nil, internalError(
			errors.New("announcement timestamps are invalid"),
			"map leading announcement",
		)
	}
	_, err := content.ParseKind(row.Kind)
	if err != nil {
		return nil, internalError(err, "validate leading announcement kind")
	}
	status, err := content.ParseStatus(row.Status)
	if err != nil {
		return nil, internalError(err, "validate leading announcement status")
	}
	if status == content.StatusDraft {
		return nil, internalError(errors.New("leading announcement is not published"), "validate leading announcement status")
	}
	action, err := mapAnnouncementAction(row)
	if err != nil {
		return nil, internalError(err, "map leading announcement action")
	}
	var endsAt *time.Time
	if row.EndsAt.Valid {
		endsAt = &row.EndsAt.Time
	}
	return &Announcement{ID: row.ID.String(), Title: row.Title, BodyMarkdown: row.BodyMarkdown, StartsAt: row.StartsAt.Time, PublishedAt: row.PublishedAt.Time, UpdatedAt: row.UpdatedAt.Time, EndsAt: endsAt, Action: action}, nil
}

func mapAnnouncementAction(row dbgen.ContentAnnouncement) (*AnnouncementAction, error) {
	actionType, err := content.ParseActionType(row.ActionType)
	if err != nil {
		return nil, err
	}
	if err := content.ValidateAction(actionType, row.ActionLabel, row.ActionPath, row.ActionExternalUrl); err != nil {
		return nil, err
	}
	switch actionType {
	case content.ActionNone:
		return nil, nil
	case content.ActionInternal:
		return &AnnouncementAction{Label: *row.ActionLabel, Href: *row.ActionPath}, nil
	case content.ActionExternal:
		return &AnnouncementAction{Label: *row.ActionLabel, Href: *row.ActionExternalUrl, External: true}, nil
	default:
		return nil, errors.New("announcement action type is invalid")
	}
}
