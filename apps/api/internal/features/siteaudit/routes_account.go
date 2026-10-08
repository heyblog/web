package siteaudit

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/platform/httpapi"
	"heyblog-api/internal/platform/ratelimit"
)

type accountOutput[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

type accountItems[T any] struct {
	Items []T `json:"items"`
}
type accountPageInput struct {
	Page string `query:"page"`
}
type accountFriendInput struct {
	ShortID       string `path:"shortId"`
	TargetShortID string `path:"targetShortId"`
}
type accountFriendRequestInput struct {
	ShortID   string `path:"shortId"`
	RequestID string `path:"requestId"`
}
type accountFriendHostInput struct {
	ShortID    string `path:"shortId"`
	TargetHost string `path:"targetHost" maxLength:"253"`
}
type accountSaved struct {
	Status string `json:"status"`
}

func RegisterAccountRoutes(api huma.API, service *AccountService, webToken string, redisClient redis.Scripter) {
	guard := huma.Middlewares{httpapi.HumaWebAuthorization(webToken)}
	mutation := huma.Middlewares{httpapi.HumaWebAuthorization(webToken), httpapi.HumaRateLimit(ratelimit.New(redisClient), ratelimit.Policy{Name: "account-site-mutation", Capacity: 20, RefillTokens: 20, RefillInterval: time.Hour})}
	operation := func(id, method, path string) huma.Operation {
		middleware := guard
		if method != http.MethodGet {
			middleware = mutation
		}
		value := submissionOperation(id, method, path, id, middleware)
		value.Tags = []string{"account sites"}
		value.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
		return value
	}
	accountRegister(api, service, operation("account-sites", "GET", "/account/sites"), func(ctx context.Context, user auth.User, _ *struct{}) (accountItems[Snapshot], error) {
		items, err := service.Sites(ctx, user)
		return accountItems[Snapshot]{Items: items}, err
	})
	accountRegister(api, service, operation("account-site", "GET", "/account/sites/{shortId}"), func(ctx context.Context, user auth.User, input *shortIDInput) (Snapshot, error) {
		return service.Site(ctx, user, input.ShortID)
	})
	accountRegister(api, service, operation("account-submissions", "GET", "/account/site-submissions"), func(ctx context.Context, user auth.User, input *accountPageInput) (accountItems[AuditListItem], error) {
		items, err := service.Submissions(ctx, user, boundedInteger(input.Page, 1, 1, 1_000_000))
		return accountItems[AuditListItem]{Items: items}, err
	})
	accountRegister(api, service, operation("account-submission", "GET", "/account/site-submissions/{auditId}"), func(ctx context.Context, user auth.User, input *auditIDInput) (Audit, error) {
		return service.Submission(ctx, user, input.AuditID)
	})
	create := operation("account-create-submission", "POST", "/account/site-submissions")
	create.DefaultStatus = http.StatusCreated
	accountRegister(api, service, create, func(ctx context.Context, user auth.User, input *bodyInput[SubmissionInput]) (SubmissionResult, error) {
		return service.Submit(ctx, user, "", "ACCOUNT_SUBMISSION", input.Body)
	})
	for _, item := range []struct{ channel, path, id string }{
		{channel: "OWNER_UPDATE", path: "/account/sites/{shortId}/updates", id: "account-update-site"},
		{channel: "OWNER_FRIEND_LINK", path: "/account/sites/{shortId}/friend-link-submissions", id: "account-create-friend"},
	} {
		op := operation(item.id, "POST", item.path)
		op.DefaultStatus = http.StatusCreated
		accountRegister(api, service, op, func(ctx context.Context, user auth.User, input *shortIDBodyInput[SubmissionInput]) (SubmissionResult, error) {
			return service.Submit(ctx, user, input.ShortID, item.channel, input.Body)
		})
	}
	accountRegister(api, service, operation("account-friends", "GET", "/account/sites/{shortId}/friend-links"), func(ctx context.Context, user auth.User, input *shortIDInput) (AccountFriends, error) {
		return service.Friends(ctx, user, input.ShortID)
	})
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		accountRegister(api, service, operation("account-friend-"+method, method, "/account/sites/{shortId}/friend-links/{targetShortId}"), func(ctx context.Context, user auth.User, input *accountFriendInput) (accountSaved, error) {
			return accountSaved{Status: "saved"}, service.SetFriend(ctx, user, input.ShortID, input.TargetShortID, method == http.MethodPut)
		})
	}
	accountRegister(api, service, operation("account-cancel-friend", "DELETE", "/account/sites/{shortId}/friend-link-requests/{requestId}"), func(ctx context.Context, user auth.User, input *accountFriendRequestInput) (accountSaved, error) {
		return accountSaved{Status: "cancelled"}, service.CancelFriend(ctx, user, input.ShortID, input.RequestID)
	})
	accountRegister(api, service, operation("account-remove-friend-host", "DELETE", "/account/sites/{shortId}/friend-links/by-host/{targetHost}"), func(ctx context.Context, user auth.User, input *accountFriendHostInput) (accountSaved, error) {
		return accountSaved{Status: "saved"}, service.RemoveFriend(ctx, user, input.ShortID, input.TargetHost)
	})
}

func accountRegister[I, O any](api huma.API, service *AccountService, operation huma.Operation, handle func(context.Context, auth.User, *I) (O, error)) {
	httpapi.Register(api, operation, func(ctx context.Context, input *I) (*accountOutput[O], error) {
		user, err := service.audits.auth.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapServiceError(err, "authorize account site operation")
		}
		result, err := handle(ctx, user, input)
		if err != nil {
			return nil, mapServiceError(accountWriteError(err), operation.OperationID)
		}
		return &accountOutput[O]{CacheControl: "private, no-store", Body: result}, nil
	})
}
