package siteaudit

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/httpapi"
	"heyblog-api/internal/ratelimit"
)

func RegisterRoutes(router *gin.Engine, service *Service, webToken string, redisClient redis.Scripter) error {
	if service == nil {
		return errors.New("site audit service is required")
	}
	guard := httpapi.WebAuthorization(webToken)
	limiter := ratelimit.New(redisClient)
	publicMutation := []httpapi.Middleware{guard, httpapi.RateLimit(limiter, ratelimit.Policy{Name: "site-audit-submit", Capacity: 6, RefillTokens: 6, RefillInterval: time.Hour})}
	publicRead := []httpapi.Middleware{guard, httpapi.RateLimit(limiter, ratelimit.Policy{Name: "site-audit-read", Capacity: 30, RefillTokens: 30, RefillInterval: time.Minute})}
	management := []httpapi.Middleware{guard}

	register := func(method, path string, endpoint httpapi.Endpoint, middleware ...httpapi.Middleware) {
		handler := httpapi.Adapt(httpapi.Chain(endpoint, middleware...))
		switch method {
		case http.MethodGet:
			router.GET(path, handler)
		case http.MethodPost:
			router.POST(path, handler)
		case http.MethodPut:
			router.PUT(path, handler)
		case http.MethodDelete:
			router.DELETE(path, handler)
		}
	}
	register(http.MethodPost, "/site-submissions", submitEndpoint(service, ActionCreate), publicMutation...)
	register(http.MethodPost, "/site-submissions/:shortId/updates", submitEndpoint(service, ActionUpdate), publicMutation...)
	register(http.MethodPost, "/site-submissions/:shortId/deletions", submitEndpoint(service, ActionDelete), publicMutation...)
	register(http.MethodPost, "/site-submissions/:shortId/restorations", submitEndpoint(service, ActionRestore), publicMutation...)
	register(http.MethodPost, "/site-submissions/query", queryEndpoint(service), publicRead...)
	register(http.MethodGet, "/site-submissions/options", optionsEndpoint(service), publicRead...)
	register(http.MethodGet, "/site-submissions/sites", searchEndpoint(service), publicRead...)
	register(http.MethodGet, "/site-submissions/sites/:shortId", resolveEndpoint(service), publicRead...)
	register(http.MethodGet, "/management/site-audits", managementListEndpoint(service), management...)
	register(http.MethodGet, "/management/site-audits/:auditId", managementDetailEndpoint(service), management...)
	register(http.MethodPut, "/management/site-audits/:auditId/review-draft", managementSaveReviewDraftEndpoint(service), management...)
	register(http.MethodDelete, "/management/site-audits/:auditId/review-draft", managementDiscardReviewDraftEndpoint(service), management...)
	register(http.MethodPost, "/management/site-audits/:auditId/review", managementReviewEndpoint(service), management...)
	return nil
}
