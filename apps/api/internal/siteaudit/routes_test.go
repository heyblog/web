package siteaudit

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesUsesShortIDForPublicMaintenancePaths(t *testing.T) {
	t.Parallel()

	router := gin.New()
	if err := RegisterRoutes(router, &Service{}, "test-web-token", nil); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	wanted := map[string]bool{
		"POST /site-submissions/:shortId/updates":      false,
		"POST /site-submissions/:shortId/deletions":    false,
		"POST /site-submissions/:shortId/restorations": false,
		"GET /site-submissions/sites/:shortId":         false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("registered routes missing %s", route)
		}
	}
}
