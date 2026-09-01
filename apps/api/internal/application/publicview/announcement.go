package publicview

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/domain/content"
)

type Announcement struct {
	Title    string              `json:"title"`
	StartsAt time.Time           `json:"startsAt"`
	Action   *AnnouncementAction `json:"action"`
}

type AnnouncementAction struct {
	Label    string `json:"label"`
	Href     string `json:"href"`
	External bool   `json:"external"`
}

func (service *Service) loadAnnouncement(ctx context.Context) (*Announcement, error) {
	row, err := service.queries.GetLeadingActiveMainAnnouncement(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internalError(err, "load leading announcement")
	}
	if !row.StartsAt.Valid {
		return nil, internalError(
			errors.New("announcement start time is invalid"),
			"map leading announcement",
		)
	}
	kind, err := content.ParseKind(row.Kind)
	if err != nil {
		return nil, internalError(err, "validate leading announcement kind")
	}
	if kind != content.KindMain {
		return nil, internalError(errors.New("leading announcement is not a main announcement"), "validate leading announcement kind")
	}
	status, err := content.ParseStatus(row.Status)
	if err != nil {
		return nil, internalError(err, "validate leading announcement status")
	}
	if status != content.StatusPublished {
		return nil, internalError(errors.New("leading announcement is not published"), "validate leading announcement status")
	}
	action, err := mapAnnouncementAction(row)
	if err != nil {
		return nil, internalError(err, "map leading announcement action")
	}
	return &Announcement{Title: row.Title, StartsAt: row.StartsAt.Time, Action: action}, nil
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
