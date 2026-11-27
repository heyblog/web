package taskcenter

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type Authenticator interface {
	Current(context.Context, *http.Request) (auth.User, error)
}
type Routes struct {
	Repository  *Repository
	Signing     *Signing
	Execution   *Execution
	Auth        Authenticator
	WebToken    string
	WorkerToken string
}
type emptyInput struct{}
type IDInput struct {
	ID string `path:"id" format:"uuid"`
}
type KeyInput struct {
	Key string `header:"Idempotency-Key" maxLength:"120"`
}
type bodyInput[T any] struct {
	KeyInput
	Body T
}
type idBodyInput[T any] struct {
	IDInput
	KeyInput
	Body T
}
type controlInput struct {
	IDInput
	KeyInput
}
type output[T any] struct {
	Status       int
	CacheControl string `header:"Cache-Control"`
	Body         T
}
type accepted struct {
	ID string `json:"id"`
}

func response[T any](value T, err error) (*output[T], error) {
	if err != nil {
		return nil, err
	}
	return &output[T]{CacheControl: "private, no-store", Body: value}, nil
}
func (r Routes) authorize(ctx context.Context, admin bool) (auth.User, error) {
	user, err := r.Auth.Current(ctx, httpapi.Request(ctx))
	if err != nil {
		var failure *auth.AuthError
		if errors.As(err, &failure) {
			kind := apperror.KindUnauthorized
			if failure.StatusCode == 403 {
				kind = apperror.KindForbidden
			}
			return user, apperror.New(kind, failure.Code, failure.Message)
		}
		return user, err
	}
	if user.Role == auth.RoleSysAdmin {
		return user, nil
	}
	if !admin && user.Role == auth.RoleAdmin && slices.Contains(user.Permissions, auth.PermissionTaskManage) {
		return user, nil
	}
	return user, apperror.New(apperror.KindForbidden, "forbidden", "task management permission is required")
}
func actor(user auth.User) Actor { return Actor{ID: user.ID, System: user.Role == auth.RoleSysAdmin} }

func (r Routes) operation(id, method, path string) huma.Operation {
	return huma.Operation{OperationID: id, Method: method, Path: "/api/web/management/" + path,
		Tags: []string{"task center"}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}},
		Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(r.WebToken)}, Errors: []int{401, 403, 404, 409, 422, 500}}
}
func RegisterRoutes(api huma.API, r Routes) {
	registerConfigurationRoutes(api, r)
	registerRunRoutes(api, r)
	registerHistoryRoutes(api, r)
	registerObservabilityRoutes(api, r)
	registerSigningRoutes(api, r)
	registerWorkerRoutes(api, r)
}

func mutationResponse[T any](ctx context.Context, r Routes, key, operation string, payload any, fn func(*Repository, Actor) (T, error)) (*output[T], error) {
	user, err := r.authorize(ctx, false)
	if err != nil {
		return nil, err
	}
	digest, err := payloadDigest(payload)
	if err != nil {
		return nil, err
	}
	return response(mutate(ctx, r.Repository, mutation[T]{Scope: user.ID + ":" + operation, Key: key, Digest: digest},
		func(repo *Repository) (T, error) { return fn(repo, actor(user)) }))
}
