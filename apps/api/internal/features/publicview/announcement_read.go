package publicview

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

type AnnouncementReader interface {
	Banner(context.Context) (*Announcement, error)
	AnnouncementArchive(context.Context, int32, int32) (AnnouncementArchive, error)
	AnnouncementByID(context.Context, string) (*Announcement, error)
}

type AnnouncementArchive struct {
	Announcements []Announcement `json:"announcements"`
	Total         int64          `json:"total"`
	Page          int32          `json:"page"`
	PageSize      int32          `json:"pageSize"`
}

type AnnouncementReadQueries interface {
	GetActiveBannerAnnouncement(context.Context) (dbgen.ContentAnnouncement, error)
	ListPublicAnnouncementArchive(context.Context, dbgen.ListPublicAnnouncementArchiveParams) ([]dbgen.ContentAnnouncement, error)
	CountPublicAnnouncementArchive(context.Context) (int64, error)
	GetPublicAnnouncementByID(context.Context, pgtype.UUID) (dbgen.ContentAnnouncement, error)
}

func (service *Service) Banner(ctx context.Context) (*Announcement, error) {
	row, err := service.announcements.GetActiveBannerAnnouncement(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internalError(err, "load banner")
	}
	return mapAnnouncement(row)
}

func (service *Service) AnnouncementArchive(ctx context.Context, page, pageSize int32) (AnnouncementArchive, error) {
	total, err := service.announcements.CountPublicAnnouncementArchive(ctx)
	if err != nil {
		return AnnouncementArchive{}, internalError(err, "count announcement archive")
	}
	rows, err := service.announcements.ListPublicAnnouncementArchive(ctx, dbgen.ListPublicAnnouncementArchiveParams{PageSize: pageSize, PageOffset: (page - 1) * pageSize})
	if err != nil {
		return AnnouncementArchive{}, internalError(err, "list announcement archive")
	}
	result := AnnouncementArchive{Announcements: []Announcement{}, Total: total, Page: page, PageSize: pageSize}
	for _, row := range rows {
		view, err := mapAnnouncement(row)
		if err != nil {
			return AnnouncementArchive{}, err
		}
		result.Announcements = append(result.Announcements, *view)
	}
	return result, nil
}

func (service *Service) AnnouncementByID(ctx context.Context, id string) (*Announcement, error) {
	var identifier pgtype.UUID
	if err := identifier.Scan(id); err != nil {
		return nil, apperror.New(apperror.KindValidation, "validation_failed", "announcement identifier is invalid")
	}
	row, err := service.announcements.GetPublicAnnouncementByID(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.New(apperror.KindNotFound, "not_found", "announcement was not found")
	}
	if err != nil {
		return nil, internalError(err, "load public announcement")
	}
	return mapAnnouncement(row)
}
