package sitestats

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/platform/httpapi"
	"heyblog-api/internal/platform/ratelimit"
)

type outboundInput struct {
	ShortID string `path:"shortId" pattern:"^[0-9A-Za-z]{9}$"`
	Body    struct {
		EventID *string `json:"eventId" format:"uuid"`
	}
}
type outboundOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		HomepageURL string `json:"homepageUrl"`
	}
}
type impressionsInput struct {
	Body struct {
		Events []Event `json:"events" minItems:"1" maxItems:"100"`
	}
}
type impressionsOutput struct {
	CacheControl string `header:"Cache-Control"`
}

func RegisterRoutes(api huma.API, service *Service, webToken string, redisClient redis.Scripter) {
	limiter := ratelimit.New(redisClient)
	guard := httpapi.HumaWebAuthorization(webToken)
	policy := func(name string) ratelimit.Policy {
		return ratelimit.Policy{Name: name, Capacity: 240, RefillTokens: 240, RefillInterval: time.Minute}
	}
	httpapi.Register(api, huma.Operation{
		OperationID: "resolve-site-outbound", Method: http.MethodPost, Path: "/sites/id/{shortId}/outbound", Summary: "Resolve a site homepage and optionally record navigation",
		Tags: []string{"site metrics"}, Security: []map[string][]string{{"webToken": {}}}, Middlewares: huma.Middlewares{guard},
		MaxBodyBytes: 1024, Errors: []int{400, 401, 404, 503},
	}, func(ctx context.Context, input *outboundInput) (*outboundOutput, error) {
		target, err := service.Outbound(ctx, input.ShortID)
		if err != nil {
			return nil, err
		}
		if input.Body.EventID != nil {
			bounded, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			native := httpapi.NativeContext(ctx)
			decision, limitErr := limiter.Allow(bounded, native.ClientIP(), policy("site-click"))
			if err := httpapi.EnforceRateLimitDecisionForRequest(ctx, decision, limitErr); err != nil {
				service.report(ctx, err, "click_limit")
			} else {
				err := service.RecordEvents(bounded, []Event{{EventID: *input.Body.EventID, ShortID: input.ShortID}}, "CLICK")
				if err != nil {
					service.report(ctx, err, "click")
				}
			}
		}
		output := &outboundOutput{CacheControl: "no-store"}
		output.Body.HomepageURL = target
		return output, nil
	})
	httpapi.Register(api, huma.Operation{
		OperationID: "record-site-impressions", Method: http.MethodPost, Path: "/site-metrics/impressions", Summary: "Record qualified site impressions",
		Tags: []string{"site metrics"}, Security: []map[string][]string{{"webToken": {}}},
		Middlewares:  huma.Middlewares{guard, httpapi.HumaRateLimit(limiter, policy("site-impressions"))},
		MaxBodyBytes: 32_000, DefaultStatus: http.StatusNoContent, Errors: []int{400, 401, 413, 422, 429, 503},
	}, func(ctx context.Context, input *impressionsInput) (*impressionsOutput, error) {
		if err := service.RecordEvents(ctx, input.Body.Events, "IMPRESSION"); err != nil {
			return nil, err
		}
		return &impressionsOutput{CacheControl: "no-store"}, nil
	})
}
