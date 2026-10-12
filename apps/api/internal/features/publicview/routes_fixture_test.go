package publicview

import (
	"heyblog-api/internal/platform/config"
	"heyblog-api/internal/platform/httpapi"
	"testing"
)

const testWebToken = "test-web-token"
const testHealthcheckToken = "test-health-token"

func testHTTPConfig() config.HTTPConfig {
	return config.HTTPConfig{MaxBodyBytes: 1 << 20, CORS: config.CORSConfig{AllowCredentials: true}}
}
func newRouterWithViews(t *testing.T, configuration config.HTTPConfig, views Reader) *httpapi.Router {
	t.Helper()
	router, err := httpapi.NewRouter(httpapi.Options{Mode: config.ModeDevelopment, HTTP: configuration, HealthcheckToken: testHealthcheckToken, WebToken: testWebToken})
	if err != nil {
		t.Fatal(err)
	}
	RegisterRoutes(router.API, testWebToken, views)
	return router
}
func newTestRouter(t *testing.T, _ *httpapi.Health) *httpapi.Router {
	return newRouterWithViews(t, testHTTPConfig(), publicViewReaderStub{})
}
