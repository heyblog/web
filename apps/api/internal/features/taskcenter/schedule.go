package taskcenter

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	edgeclient "heyblog.local/edgerequest"

	"github.com/google/uuid"
	"golang.org/x/net/http/httpguts"
)

func (input ScheduleInput) Validate(now time.Time) (time.Time, error) {
	c := input.Configuration
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 120 || (input.Kind != "monitor" && input.Kind != "rss") {
		return time.Time{}, fmt.Errorf("invalid schedule name or kind")
	}
	if c.Scope != "all" && c.Scope != "selected" || c.Scope == "selected" && len(c.SiteIDs) == 0 || len(c.SiteIDs) > 10000 {
		return time.Time{}, fmt.Errorf("invalid target scope")
	}
	for _, id := range c.SiteIDs {
		if _, err := uuid.Parse(id); err != nil {
			return time.Time{}, fmt.Errorf("invalid site identifier")
		}
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 30 || c.IntervalMS < 0 || c.IntervalMS > 60000 || c.Concurrency < 1 || c.Concurrency > 16 || c.Retries < 0 || c.Retries > 2 {
		return time.Time{}, fmt.Errorf("request limits are invalid")
	}
	if len(c.UserAgent) < 1 || len(c.UserAgent) > 256 || !httpguts.ValidHeaderFieldValue(c.UserAgent) {
		return time.Time{}, fmt.Errorf("invalid user agent")
	}
	if len(c.Headers) > 16 {
		return time.Time{}, fmt.Errorf("too many headers")
	}
	headers := make(http.Header, len(c.Headers))
	for key, value := range c.Headers {
		if !httpguts.ValidHeaderFieldName(key) || !httpguts.ValidHeaderFieldValue(value) || len(value) > 1024 {
			return time.Time{}, fmt.Errorf("invalid request header")
		}
		switch http.CanonicalHeaderKey(key) {
		case "Authorization", "Cookie", "Host", "Connection", "Content-Length", "Transfer-Encoding", "Proxy-Authorization", "User-Agent", "X-Heyblog-Proof", "X-Web-Token":
			return time.Time{}, fmt.Errorf("reserved request header")
		}
		if strings.HasPrefix(strings.ToLower(key), "x-heyblog-") {
			return time.Time{}, fmt.Errorf("reserved request header")
		}
		headers.Set(key, value)
	}
	if _, err := edgeclient.SafeHeaders(headers); err != nil {
		return time.Time{}, fmt.Errorf("unsupported request header")
	}
	return c.Next(now)
}
