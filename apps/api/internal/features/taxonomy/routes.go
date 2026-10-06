package taxonomy

import (
	"context"
	"errors"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type bodyInput[T any] struct{ Body T }
type pathInput[T any] struct {
	ID   string `path:"id"`
	Body T
}
type output[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

func RegisterRoutes(api huma.API, s *Service, webToken string) error {
	if s == nil {
		return errors.New("taxonomy service is required")
	}
	guard := httpapi.HumaWebAuthorization(webToken)
	operation := func(id, method, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: "/management/taxonomy" + path, Tags: []string{"taxonomy management"}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, MaxBodyBytes: 65536, Middlewares: huma.Middlewares{guard}, Errors: []int{401, 403, 409, 422, 500}}
	}
	httpapi.Register(api, operation("list-managed-tags", http.MethodGet, "/tags"), func(ctx context.Context, _ *struct{}) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.List(ctx)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("create-managed-tag", http.MethodPost, "/tags"), func(ctx context.Context, input *bodyInput[CreateInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.Create(ctx, input.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("create-managed-cascade", http.MethodPost, "/cascades"), func(ctx context.Context, input *bodyInput[CreateCascadeInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.CreateCascade(ctx, input.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("update-managed-tag", http.MethodPut, "/tags/{id}"), func(ctx context.Context, input *pathInput[UpdateInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.Update(ctx, input.ID, input.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("delete-managed-tag", http.MethodDelete, "/tags/{id}"), func(ctx context.Context, input *pathInput[DeleteInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.Delete(ctx, input.ID, input.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("preview-taxonomy-change", http.MethodPost, "/changes/preview"), func(ctx context.Context, input *bodyInput[ChangeInput]) (*output[Preview], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.Preview(ctx, input.Body)
		return &output[Preview]{CacheControl: "no-store", Body: body}, err
	})
	httpapi.Register(api, operation("apply-taxonomy-change", http.MethodPost, "/changes/apply"), func(ctx context.Context, input *bodyInput[ChangeInput]) (*output[Catalog], error) {
		if err := s.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		body, err := s.Apply(ctx, input.Body)
		return &output[Catalog]{CacheControl: "no-store", Body: body}, err
	})
	registerLabelRoutes(api, s, operation)
	return nil
}
