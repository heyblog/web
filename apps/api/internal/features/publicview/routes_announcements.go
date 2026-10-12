package publicview

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/platform/httpapi"
)

type announcementPublicInput struct {
	ID string `path:"id"`
}
type announcementPageInput struct {
	Page     int32 `query:"page" default:"1" minimum:"1" maximum:"21474836"`
	PageSize int32 `query:"pageSize" default:"20" minimum:"1" maximum:"100"`
}
type announcementPublicBody struct {
	Announcement *Announcement `json:"announcement"`
}

func registerAnnouncementReadRoutes(api huma.API, webToken string, reader Reader) {
	operation := func(id, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: path, Tags: []string{"public views"}, Security: []map[string][]string{{"webToken": {}}}, Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken)}, Errors: []int{401, 404, 422, 500}}
	}
	httpapi.Register(api, operation("get-announcement-banner", "/api/web/public/announcements/banner"), func(ctx context.Context, _ *publicViewInput) (*publicViewOutput[announcementPublicBody], error) {
		view, err := reader.Banner(ctx)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[announcementPublicBody]{CacheControl: "no-store", Body: announcementPublicBody{view}}, nil
	})
	httpapi.Register(api, operation("list-announcements", "/api/web/public/announcements"), func(ctx context.Context, input *announcementPageInput) (*publicViewOutput[AnnouncementArchive], error) {
		view, err := reader.AnnouncementArchive(ctx, input.Page, input.PageSize)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[AnnouncementArchive]{CacheControl: "no-store", Body: view}, nil
	})
	httpapi.Register(api, operation("get-announcement", "/api/web/public/announcements/{id}"), func(ctx context.Context, input *announcementPublicInput) (*publicViewOutput[announcementPublicBody], error) {
		view, err := reader.AnnouncementByID(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		return &publicViewOutput[announcementPublicBody]{CacheControl: "no-store", Body: announcementPublicBody{view}}, nil
	})
}
