package announcement

import (
	"context"
	"errors"
	"net/http"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type idInput struct {
	ID string `path:"id"`
}
type idBodyInput[T any] struct {
	ID   string `path:"id"`
	Body T
}
type createInput struct{ Body Input }
type output[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}
type detailBody struct {
	ManagedAnnouncement ManagedAnnouncement `json:"announcement"`
}
type revisionsBody struct {
	Revisions []Revision `json:"revisions"`
}
type deleteOutput struct {
	Status       int    `status:"204"`
	CacheControl string `header:"Cache-Control"`
}

func RegisterRoutes(api huma.API, service *Service, webToken string) error {
	if service == nil {
		return errors.New("announcement service is required")
	}
	operation := func(id, method, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{"announcement management"}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken)}, Errors: []int{401, 403, 404, 409, 422, 500}}
	}
	httpapi.Register(api, operation("list-managed-announcements", http.MethodGet, "/management/announcements"), func(ctx context.Context, input *ListQuery) (*output[List], error) {
		if _, err := service.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		view, err := service.repository.List(ctx, *input)
		if err != nil {
			return nil, err
		}
		return &output[List]{CacheControl: "private, no-store", Body: view}, nil
	})
	createOperation := operation("create-announcement", http.MethodPost, "/management/announcements")
	createOperation.DefaultStatus = http.StatusCreated
	httpapi.Register(api, createOperation, func(ctx context.Context, input *createInput) (*output[detailBody], error) {
		user, err := service.authorize(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, err
		}
		if err := validateInput(input.Body); err != nil {
			return nil, err
		}
		view, err := service.repository.Create(ctx, user.ID, input.Body)
		if err != nil {
			return nil, err
		}
		return &output[detailBody]{CacheControl: "private, no-store", Body: detailBody{view}}, nil
	})
	httpapi.Register(api, operation("get-managed-announcement", http.MethodGet, "/management/announcements/{id}"), func(ctx context.Context, input *idInput) (*output[detailBody], error) {
		if _, err := service.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		view, err := service.repository.Get(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		return &output[detailBody]{CacheControl: "private, no-store", Body: detailBody{view}}, nil
	})
	httpapi.Register(api, operation("get-announcement-revisions", http.MethodGet, "/management/announcements/{id}/revisions"), func(ctx context.Context, input *idInput) (*output[revisionsBody], error) {
		if _, err := service.authorize(ctx, httpapi.Request(ctx)); err != nil {
			return nil, err
		}
		view, err := service.repository.Revisions(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		return &output[revisionsBody]{CacheControl: "private, no-store", Body: revisionsBody{view}}, nil
	})
	httpapi.Register(api, operation("edit-announcement", http.MethodPut, "/management/announcements/{id}"), editHandler(service))
	httpapi.Register(api, operation("publish-announcement", http.MethodPost, "/management/announcements/{id}/publish"), publishHandler(service))
	httpapi.Register(api, operation("archive-announcement", http.MethodPost, "/management/announcements/{id}/archive"), archiveHandler(service))
	deleteOperation := operation("delete-announcement", http.MethodDelete, "/management/announcements/{id}")
	deleteOperation.DefaultStatus = http.StatusNoContent
	httpapi.Register(api, deleteOperation, deleteHandler(service))
	return nil
}
