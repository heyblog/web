package announcement

import (
	"context"
	"heyblog-api/internal/platform/httpapi"
)

func editHandler(service *Service) func(context.Context, *idBodyInput[EditInput]) (*output[detailBody], error) {
	return func(ctx context.Context, input *idBodyInput[EditInput]) (*output[detailBody], error) {
		user, err := service.authorize(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, err
		}
		if err := validateInput(input.Body.Input); err != nil {
			return nil, err
		}
		version, err := parseVersion(input.Body.RowVersion)
		if err != nil {
			return nil, err
		}
		view, err := service.repository.mutate(ctx, mutation{kind: mutationEdit, id: input.ID, actor: user.ID, version: version, input: input.Body.Input})
		if err != nil {
			return nil, err
		}
		return &output[detailBody]{CacheControl: "private, no-store", Body: detailBody{view}}, nil
	}
}
func publishHandler(service *Service) func(context.Context, *idBodyInput[PublishInput]) (*output[detailBody], error) {
	return func(ctx context.Context, input *idBodyInput[PublishInput]) (*output[detailBody], error) {
		user, err := service.authorize(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, err
		}
		version, err := parseVersion(input.Body.RowVersion)
		if err != nil {
			return nil, err
		}
		view, err := service.repository.mutate(ctx, mutation{kind: mutationPublish, id: input.ID, actor: user.ID, version: version, publish: input.Body})
		if err != nil {
			return nil, err
		}
		return &output[detailBody]{CacheControl: "private, no-store", Body: detailBody{view}}, nil
	}
}
func archiveHandler(service *Service) func(context.Context, *idBodyInput[VersionInput]) (*output[detailBody], error) {
	return func(ctx context.Context, input *idBodyInput[VersionInput]) (*output[detailBody], error) {
		user, err := service.authorize(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, err
		}
		version, err := parseVersion(input.Body.RowVersion)
		if err != nil {
			return nil, err
		}
		view, err := service.repository.mutate(ctx, mutation{kind: mutationArchive, id: input.ID, actor: user.ID, version: version})
		if err != nil {
			return nil, err
		}
		return &output[detailBody]{CacheControl: "private, no-store", Body: detailBody{view}}, nil
	}
}
func deleteHandler(service *Service) func(context.Context, *idBodyInput[VersionInput]) (*deleteOutput, error) {
	return func(ctx context.Context, input *idBodyInput[VersionInput]) (*deleteOutput, error) {
		user, err := service.authorize(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, err
		}
		version, err := parseVersion(input.Body.RowVersion)
		if err != nil {
			return nil, err
		}
		if _, err := service.repository.mutate(ctx, mutation{kind: mutationDelete, id: input.ID, actor: user.ID, version: version}); err != nil {
			return nil, err
		}
		return &deleteOutput{Status: 204, CacheControl: "private, no-store"}, nil
	}
}
