package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/features/publicview"
)

type announcementPublicInput struct {
	ID string `path:"id"`
}
type announcementPageInput struct {
	Page     int32 `query:"page" default:"1" minimum:"1" maximum:"21474836"`
	PageSize int32 `query:"pageSize" default:"20" minimum:"1" maximum:"100"`
}
type announcementPublicBody struct {
	Announcement *publicview.Announcement `json:"announcement"`
}

func registerAnnouncementReadRoutes(api huma.API, webToken string, reader publicview.Reader) {
	operation := func(id, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: path, Tags: []string{"public views"}, Security: []map[string][]string{{"webToken": {}}}, Middlewares: huma.Middlewares{HumaWebAuthorization(webToken)}, Errors: []int{401, 404, 422, 500}}
	}
	Register(api, operation("get-announcement-banner", "/announcements/banner"), func(ctx context.Context, _ *publicViewInput) (*publicViewOutput[announcementPublicBody], error) {
		view, err := reader.Banner(ctx)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[announcementPublicBody]{CacheControl: "no-store", Body: announcementPublicBody{view}}, nil
	})
	Register(api, operation("list-announcements", "/announcements"), func(ctx context.Context, input *announcementPageInput) (*publicViewOutput[publicview.AnnouncementArchive], error) {
		view, err := reader.AnnouncementArchive(ctx, input.Page, input.PageSize)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[publicview.AnnouncementArchive]{CacheControl: "no-store", Body: view}, nil
	})
	Register(api, operation("get-announcement", "/announcements/{id}"), func(ctx context.Context, input *announcementPublicInput) (*publicViewOutput[announcementPublicBody], error) {
		view, err := reader.AnnouncementByID(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[announcementPublicBody]{CacheControl: "no-store", Body: announcementPublicBody{view}}, nil
	})
}
