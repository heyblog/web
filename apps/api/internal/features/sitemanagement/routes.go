package sitemanagement

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/httpapi"
	"heyblog-api/internal/platform/ratelimit"
)

type output[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

func response[T any](value T) *output[T] {
	return &output[T]{CacheControl: "private, no-store", Body: value}
}

type createRequest struct{ Body SiteClaimCreateInput }
type claimRequest struct {
	ID string `path:"claimId"`
}
type reviewRequest struct {
	ID   string `path:"claimId"`
	Body SiteClaimReviewInput
}
type ownerRequest struct {
	ShortID string `path:"shortId"`
	Body    OwnershipInput
}
type listRequest struct {
	Page string `query:"page"`
	Size string `query:"page_size"`
}
type claimList struct {
	Items []Claim `json:"items"`
}
type success struct {
	Success bool `json:"success"`
}

func RegisterRoutes(api huma.API, service *Service, webToken string, redisClient redis.Scripter) error {
	if service == nil {
		return errors.New("site management service is required")
	}
	limiter := ratelimit.New(redisClient)
	operation := func(id, method, path string) huma.Operation {
		policy := ratelimit.Policy{Name: "site-claims-read", Capacity: 30, RefillTokens: 30, RefillInterval: time.Minute}
		if method != http.MethodGet {
			policy = ratelimit.Policy{Name: "site-claims-write", Capacity: 10, RefillTokens: 10, RefillInterval: time.Hour}
		}
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: id, Tags: []string{"site management"}, Errors: []int{400, 401, 403, 404, 409, 422, 429, 503}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), httpapi.HumaRateLimit(limiter, policy)}}
	}
	httpapi.Register(api, operation("create-site-claim", http.MethodPost, "/account/site-claims"), func(ctx context.Context, input *createRequest) (*output[CreateResult], error) {
		user, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		result, err := service.Create(ctx, user, input.Body)
		if err != nil {
			return nil, routeError(err)
		}
		return response(result), nil
	})
	for _, management := range []bool{false, true} {
		path := "/account/site-claims"
		id := "account-site-claims"
		if management {
			path = "/management/site-claims"
			id = "management-site-claims"
		}
		httpapi.Register(api, operation("list-"+id, http.MethodGet, path), listHandler(service, management))
		httpapi.Register(api, operation("get-"+id, http.MethodGet, path+"/{claimId}"), detailHandler(service, management))
	}
	httpapi.Register(api, operation("check-site-claim", http.MethodPost, "/account/site-claims/{claimId}/check"), claimActionHandler(service, false))
	httpapi.Register(api, operation("cancel-site-claim", http.MethodDelete, "/account/site-claims/{claimId}"), claimActionHandler(service, true))
	httpapi.Register(api, operation("review-site-claim", http.MethodPost, "/management/site-claims/{claimId}/review"), func(ctx context.Context, input *reviewRequest) (*output[Claim], error) {
		user, err := service.Reviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		result, err := service.Review(ctx, user, input.ID, input.Body)
		if err != nil {
			return nil, routeError(err)
		}
		return response(result), nil
	})
	httpapi.Register(api, operation("revoke-site-ownership", http.MethodDelete, "/management/site-ownership/{shortId}"), ownerHandler(service, true))
	httpapi.Register(api, operation("reassign-site-ownership", http.MethodPut, "/management/site-ownership/{shortId}"), ownerHandler(service, false))
	return nil
}
func listHandler(s *Service, management bool) func(context.Context, *listRequest) (*output[claimList], error) {
	return func(ctx context.Context, input *listRequest) (*output[claimList], error) {
		user, err := s.Current(ctx, httpapi.Request(ctx))
		if management {
			user, err = s.Reviewer(ctx, httpapi.Request(ctx))
		}
		if err != nil {
			return nil, routeError(err)
		}
		userID := user.ID
		if management {
			userID = ""
		}
		page := integer(input.Page, 1, 1000000)
		size := integer(input.Size, 20, 50)
		items, err := s.store.List(ctx, userID, size, (page-1)*size)
		if err != nil {
			return nil, routeError(err)
		}
		return response(claimList{Items: items}), nil
	}
}
func detailHandler(s *Service, management bool) func(context.Context, *claimRequest) (*output[Claim], error) {
	return func(ctx context.Context, input *claimRequest) (*output[Claim], error) {
		user, err := s.Current(ctx, httpapi.Request(ctx))
		if management {
			user, err = s.Reviewer(ctx, httpapi.Request(ctx))
		}
		if err != nil {
			return nil, routeError(err)
		}
		claim, err := s.store.Claim(ctx, input.ID)
		if err != nil {
			return nil, routeError(err)
		}
		if !management && claim.UserID != user.ID {
			return nil, routeError(ErrNotFound)
		}
		return response(claim), nil
	}
}
func claimActionHandler(s *Service, cancel bool) func(context.Context, *claimRequest) (*output[Claim], error) {
	return func(ctx context.Context, input *claimRequest) (*output[Claim], error) {
		user, err := s.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		var claim Claim
		if cancel {
			claim, err = s.Cancel(ctx, user, input.ID)
		} else {
			claim, err = s.Check(ctx, user, input.ID)
		}
		if err != nil {
			return nil, routeError(err)
		}
		return response(claim), nil
	}
}
func ownerHandler(s *Service, revoke bool) func(context.Context, *ownerRequest) (*output[success], error) {
	return func(ctx context.Context, input *ownerRequest) (*output[success], error) {
		user, err := s.Reviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		if revoke {
			input.Body.UserID = ""
		} else if input.Body.UserID == "" {
			return nil, routeError(failure("invalid_user", "An account is required for assignment", 422))
		}
		if err := s.ChangeOwner(ctx, user, input.ShortID, input.Body); err != nil {
			return nil, routeError(err)
		}
		return response(success{Success: true}), nil
	}
}
func integer(raw string, fallback, maximum int32) int32 {
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < 1 || value > int64(maximum) {
		return fallback
	}
	return int32(value)
}
func routeError(err error) error {
	var authentication *auth.AuthError
	if errors.As(err, &authentication) {
		return huma.NewError(authentication.StatusCode, authentication.Message)
	}
	if errors.Is(err, ErrNotFound) {
		return apperror.New(apperror.KindNotFound, "claim_not_found", "The site or verification application was not found")
	}
	var service *ServiceError
	if errors.As(err, &service) {
		kind := apperror.KindValidation
		switch service.Status {
		case 403:
			kind = apperror.KindForbidden
		case 409:
			kind = apperror.KindConflict
		}
		return apperror.Wrap(err, kind, service.Code, service.Message, "site management")
	}
	return apperror.Wrap(err, apperror.KindInternal, apperror.CodeInternal, "The site verification operation failed", "site management")
}
