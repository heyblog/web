package siteaudit

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/httpapi"
)

const annotationPageSize = 20

type annotationListInput struct {
	Query  string `query:"q" maxLength:"160"`
	Status string `query:"status" enum:"UNREVIEWED,BLOG,NON_BLOG,ALL"`
	Page   string `query:"page"`
}

type annotationHistoryInput struct {
	ShortID string `path:"shortId"`
	Page    string `query:"page"`
}

func RegisterAnnotationRoutes(api huma.API, repository *Repository, authentication *auth.Service, webToken string) {
	const path = "/api/web/management/site-annotations"
	operation := func(id, method, suffix, summary string) huma.Operation {
		value := submissionOperation(id, method, path+suffix, summary, huma.Middlewares{httpapi.HumaWebAuthorization(webToken)})
		value.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
		value.Tags = []string{"site annotations"}
		value.MaxBodyBytes = 256 << 10
		return value
	}

	registerAnnotationOperation(api, operation("list-site-annotations", http.MethodGet, "", "List sites for annotation"), authentication,
		func(ctx context.Context, input *annotationListInput, _ auth.User) (annotationPage[AnnotationSite], error) {
			status := input.Status
			if status == "" {
				status = string(site.BlogUnreviewed)
			}
			return repository.annotationSites(ctx, annotationFilter{
				Status: status, Query: strings.TrimSpace(input.Query), Page: boundedInteger(input.Page, 1, 1, 1_000_000),
			})
		})
	registerAnnotationOperation(api, operation("get-site-annotation-options", http.MethodGet, "/options", "List annotation options"), authentication,
		func(ctx context.Context, _ *struct{}, _ auth.User) (SubmissionOptions, error) {
			return repository.Options(ctx)
		})
	registerAnnotationOperation(api, operation("get-site-annotation", http.MethodGet, "/{shortId}", "Get site annotation detail"), authentication,
		func(ctx context.Context, input *shortIDInput, _ auth.User) (AnnotationDetail, error) {
			return repository.annotationDetail(ctx, input.ShortID)
		})
	registerAnnotationOperation(api, operation("update-site-annotation", http.MethodPatch, "/{shortId}", "Save site annotation"), authentication,
		func(ctx context.Context, input *shortIDBodyInput[AnnotationPatch], actor auth.User) (AnnotationDetail, error) {
			return repository.saveAnnotation(ctx, annotationChange{ShortID: input.ShortID, ActorID: actor.ID, Patch: input.Body})
		})
	registerAnnotationOperation(api, operation("list-site-annotation-history", http.MethodGet, "/{shortId}/history", "List site annotation history"), authentication,
		func(ctx context.Context, input *annotationHistoryInput, _ auth.User) (annotationPage[AnnotationHistory], error) {
			return repository.annotationHistory(ctx, input.ShortID, boundedInteger(input.Page, 1, 1, 1_000_000))
		})
}

type annotationResponse[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

func registerAnnotationOperation[I, T any](
	api huma.API, operation huma.Operation, authentication *auth.Service,
	handle func(context.Context, *I, auth.User) (T, error),
) {
	httpapi.Register(api, operation, func(ctx context.Context, input *I) (*annotationResponse[T], error) {
		actor, err := authentication.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapServiceError(err, operation.OperationID)
		}
		if actor.Role != auth.RoleSysAdmin && (actor.Role != auth.RoleAdmin || !slices.Contains(actor.Permissions, auth.PermissionSiteAnnotation)) {
			return nil, mapServiceError(newServiceError("forbidden", http.StatusForbidden, "site annotation permission is required"), operation.OperationID)
		}
		value, err := handle(ctx, input, actor)
		if err != nil {
			return nil, mapServiceError(err, operation.OperationID)
		}
		return &annotationResponse[T]{CacheControl: "private, no-store", Body: value}, nil
	})
}
