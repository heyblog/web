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
	Page   string `query:"page"`
	Status string `query:"status"`
	Method string `query:"method"`
	Size   string `query:"page_size"`
}
type siteRequest struct {
	ShortID string `path:"shortId" minLength:"9" maxLength:"9"`
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
		policy := claimRatePolicy(id)
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: id, Tags: []string{"site management"}, Errors: []int{400, 401, 403, 404, 409, 422, 429, 503}, Security: []map[string][]string{{"webToken": {}, "accessCookie": {}}}, Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(webToken), httpapi.HumaRateLimit(limiter, policy)}}
	}
	httpapi.Register(api, operation("create-site-claim", http.MethodPost, "/api/web/account/site-claims"), func(ctx context.Context, input *createRequest) (*output[CreateResult], error) {
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
		path := "/api/web/account/site-claims"
		id := "account-site-claims"
		if management {
			path = "/api/web/management/site-claims"
			id = "management-site-claims"
		}
		httpapi.Register(api, operation("list-"+id, http.MethodGet, path), listHandler(service, management))
		httpapi.Register(api, operation("get-"+id, http.MethodGet, path+"/{claimId}"), detailHandler(service, management))
	}
	httpapi.Register(api, operation("check-site-claim", http.MethodPost, "/api/web/account/site-claims/{claimId}/check"), claimActionHandler(service, false))
	httpapi.Register(api, operation("cancel-site-claim", http.MethodDelete, "/api/web/account/site-claims/{claimId}"), claimActionHandler(service, true))
	httpapi.Register(api, operation("review-site-claim", http.MethodPost, "/api/web/management/site-claims/{claimId}/review"), func(ctx context.Context, input *reviewRequest) (*output[Claim], error) {
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
	httpapi.Register(api, operation("revoke-site-ownership", http.MethodDelete, "/api/web/management/site-ownership/{shortId}"), ownerHandler(service))
	httpapi.Register(api, operation("site-claim-context", http.MethodGet, "/api/web/account/sites/{shortId}/claim-context"), func(ctx context.Context, input *siteRequest) (*output[ClaimContext], error) {
		user, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		result, err := service.Context(ctx, user, input.ShortID)
		if err != nil {
			return nil, routeError(err)
		}
		return response(result), nil
	})
	httpapi.Register(api, operation("get-site-ownership", http.MethodGet, "/api/web/management/site-ownership/{shortId}"), func(ctx context.Context, input *siteRequest) (*output[OwnershipDetail], error) {
		user, err := service.Reviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		result, err := service.Ownership(ctx, user, input.ShortID)
		if err != nil {
			return nil, routeError(err)
		}
		return response(result), nil
	})
	httpapi.Register(api, operation("regenerate-site-claim", http.MethodPost, "/api/web/account/site-claims/{claimId}/regenerate"), func(ctx context.Context, input *claimRequest) (*output[CreateResult], error) {
		user, err := service.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		result, err := service.Regenerate(ctx, user, input.ID)
		if err != nil {
			return nil, routeError(err)
		}
		return response(result), nil
	})
	return nil
}
func listHandler(s *Service, management bool) func(context.Context, *listRequest) (*output[ClaimList], error) {
	return func(ctx context.Context, input *listRequest) (*output[ClaimList], error) {
		user, err := routeUser(ctx, s, management)
		if err != nil {
			return nil, routeError(err)
		}
		page := integer(input.Page, 1, 1000000)
		size := integer(input.Size, 20, 50)
		result, err := s.List(ctx, user, ClaimFilter{Status: input.Status, Method: input.Method, Limit: size, Offset: (page - 1) * size}, management)
		if err != nil {
			return nil, routeError(err)
		}
		return response(result), nil
	}
}
func detailHandler(s *Service, management bool) func(context.Context, *claimRequest) (*output[Claim], error) {
	return func(ctx context.Context, input *claimRequest) (*output[Claim], error) {
		user, err := routeUser(ctx, s, management)
		if err != nil {
			return nil, routeError(err)
		}
		claim, err := s.Detail(ctx, user, input.ID, management)
		if err != nil {
			return nil, routeError(err)
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
func ownerHandler(s *Service) func(context.Context, *ownerRequest) (*output[success], error) {
	return func(ctx context.Context, input *ownerRequest) (*output[success], error) {
		user, err := s.Reviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}

		if err := s.Revoke(ctx, user, input.ShortID, input.Body); err != nil {
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
	var application *apperror.Error
	if errors.As(err, &application) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return apperror.Wrap(err, apperror.KindUnavailable, "service_unavailable", "Site verification is temporarily unavailable", "site management")
	}
	var authentication *auth.AuthError
	if errors.As(err, &authentication) {
		kind := apperror.KindUnauthorized
		if authentication.StatusCode == 403 {
			kind = apperror.KindForbidden
		}
		return apperror.Wrap(err, kind, authentication.Code, authentication.Message, "site management")
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

func routeUser(ctx context.Context, s *Service, management bool) (auth.User, error) {
	if management {
		return s.Reviewer(ctx, httpapi.Request(ctx))
	}
	return s.Current(ctx, httpapi.Request(ctx))
}
func claimRatePolicy(id string) ratelimit.Policy {
	name, capacity, interval := "site-claims-read", int64(30), time.Minute
	switch id {
	case "create-site-claim", "regenerate-site-claim":
		name, capacity, interval = "site-claims-create", 10, time.Hour
	case "check-site-claim":
		name, capacity, interval = "site-claims-check", 30, time.Minute
	case "cancel-site-claim":
		name, capacity, interval = "site-claims-cancel", 30, time.Hour
	case "review-site-claim", "revoke-site-ownership":
		name, capacity, interval = "site-claims-review", 60, time.Hour
	}
	return ratelimit.Policy{Name: name, Capacity: capacity, RefillTokens: capacity, RefillInterval: interval}
}
