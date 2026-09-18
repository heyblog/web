package siteaudit

import (
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/ratelimit"
)

func RegisterRoutes(api huma.API, service *Service, webToken string, redisClient redis.Scripter) error {
	if service == nil {
		return errors.New("site audit service is required")
	}
	limiter := ratelimit.New(redisClient)
	guard := httpapi.HumaWebAuthorization(webToken)
	mutation := huma.Middlewares{
		guard,
		httpapi.HumaRateLimit(limiter, ratelimit.Policy{
			Name: "site-audit-submit", Capacity: 6, RefillTokens: 6, RefillInterval: time.Hour,
		}),
	}
	read := huma.Middlewares{
		guard,
		httpapi.HumaRateLimit(limiter, ratelimit.Policy{
			Name: "site-audit-read", Capacity: 30, RefillTokens: 30, RefillInterval: time.Minute,
		}),
	}
	registerSubmissionRoutes(api, service, mutation, read)
	registerAuditManagementRoutes(api, service, huma.Middlewares{guard})
	return nil
}

func submissionOperation(id, method, path, summary string, middleware huma.Middlewares) huma.Operation {
	return huma.Operation{
		OperationID: id,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Tags:        []string{"site submissions"},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusTooManyRequests,
			http.StatusServiceUnavailable,
		},
		Security:    []map[string][]string{{"webToken": {}}},
		Middlewares: middleware,
	}
}

func registerSubmissionRoutes(api huma.API, service *Service, mutation, read huma.Middlewares) {
	createOperation := submissionOperation("create-site-submission", http.MethodPost, "/site-submissions", "Create a site submission", mutation)
	createOperation.DefaultStatus = http.StatusCreated
	httpapi.Register(api, createOperation, createSubmitHandler(service))

	for _, route := range []struct {
		id      string
		path    string
		summary string
		action  Action
	}{
		{id: "update-site-submission", path: "/site-submissions/{shortId}/updates", summary: "Submit a site update", action: ActionUpdate},
		{id: "delete-site-submission", path: "/site-submissions/{shortId}/deletions", summary: "Submit a site deletion", action: ActionDelete},
		{id: "restore-site-submission", path: "/site-submissions/{shortId}/restorations", summary: "Submit a site restoration", action: ActionRestore},
	} {
		operation := submissionOperation(route.id, http.MethodPost, route.path, route.summary, mutation)
		operation.DefaultStatus = http.StatusCreated
		httpapi.Register(api, operation, submitHandler(service, route.action))
	}

	httpapi.Register(api, submissionOperation("query-site-submission", http.MethodPost, "/site-submissions/query", "Query a site submission", read), queryHandler(service))
	httpapi.Register(api, submissionOperation("get-site-submission-options", http.MethodGet, "/site-submissions/options", "Get site submission options", read), optionsHandler(service))
	httpapi.Register(api, submissionOperation("check-site-availability", http.MethodGet, "/site-submissions/site-availability", "Check site availability", read), availabilityHandler(service))
	httpapi.Register(api, submissionOperation("search-submission-sites", http.MethodGet, "/site-submissions/sites", "Search sites for a submission", read), searchHandler(service))
	httpapi.Register(api, submissionOperation("resolve-submission-site", http.MethodGet, "/site-submissions/sites/{shortId}", "Resolve a site for a submission", read), resolveHandler(service))
}

func registerAuditManagementRoutes(api huma.API, service *Service, middleware huma.Middlewares) {
	operation := func(id, method, path, summary string) huma.Operation {
		value := submissionOperation(id, method, path, summary, middleware)
		value.Tags = []string{"site audit management"}
		value.Security = []map[string][]string{{"webToken": {}, "accessCookie": {}}}
		return value
	}
	httpapi.Register(api, operation("list-site-audits", http.MethodGet, "/management/site-audits", "List site audits"), managementListHandler(service))
	httpapi.Register(api, operation("get-site-audit", http.MethodGet, "/management/site-audits/{auditId}", "Get a site audit"), managementDetailHandler(service))
	httpapi.Register(api, operation("save-site-audit-review-draft", http.MethodPut, "/management/site-audits/{auditId}/review-draft", "Save a site audit review draft"), managementSaveReviewDraftHandler(service))
	httpapi.Register(api, operation("discard-site-audit-review-draft", http.MethodDelete, "/management/site-audits/{auditId}/review-draft", "Discard a site audit review draft"), managementDiscardReviewDraftHandler(service))
	httpapi.Register(api, operation("review-site-audit", http.MethodPost, "/management/site-audits/{auditId}/review", "Review a site audit"), managementReviewHandler(service))
}
