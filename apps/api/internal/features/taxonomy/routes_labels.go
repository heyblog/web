package taxonomy

import (
	"context"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type labelPathInput[T any] struct {
	ID      string `path:"id"`
	LabelID string `path:"label_id"`
	Body    T
}

func registerLabelRoutes(api huma.API, s *Service, operation func(string, string, string) huma.Operation) {
	httpapi.Register(api, operation("create-managed-tag-label", http.MethodPost, "/tags/{id}/labels"), func(ctx context.Context, in *pathInput[LabelInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.CreateLabel(ctx, in.ID, in.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("update-managed-tag-label", http.MethodPut, "/tags/{id}/labels/{label_id}"), func(ctx context.Context, in *labelPathInput[LabelInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.UpdateLabel(ctx, in.ID, in.LabelID, in.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("delete-managed-tag-label", http.MethodDelete, "/tags/{id}/labels/{label_id}"), func(ctx context.Context, in *labelPathInput[DeleteInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.DeleteLabel(ctx, in.ID, in.LabelID, in.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("set-managed-tag-default-label", http.MethodPost, "/tags/{id}/default-label"), func(ctx context.Context, in *pathInput[DefaultLabelInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.DefaultLabel(ctx, in.ID, in.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
}
