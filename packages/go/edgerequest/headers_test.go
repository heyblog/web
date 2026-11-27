package edgerequest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeHeaders_when_FieldCannotCrossEdgeBoundary(t *testing.T) {
	for _, headers := range []http.Header{
		{"X-" + strings.Repeat("a", 127): {"value"}},
		{"X-Invalid Name": {"value"}},
		{"If-None-Match": {"value\x01"}},
		{"User-Agent": {"value\x7f"}},
	} {
		// Given a field rejected by the Edge contract or HTTP field syntax.
		// When it passes through the common request boundary.
		_, err := SafeHeaders(headers)
		// Then invalid fields cannot be dispatched or saved as task configuration.
		var failure *Error
		require.ErrorAs(t, err, &failure)
		require.Equal(t, Security, failure.Kind)
	}
}

func TestSafeHeaders_when_UTF8FieldIsWithinLimits(t *testing.T) {
	// Given an existing UTF-8 UA and a header name at the Edge limit.
	name := "X-" + strings.Repeat("a", 126)
	headers := http.Header{"User-Agent": {"测试UA"}, name: {"value"}}
	// When preparing the Edge JSON envelope.
	result, err := SafeHeaders(headers)
	// Then accepted text is preserved for the Edge's UTF-8 byte conversion.
	require.NoError(t, err)
	require.Equal(t, "测试UA", result.Get("User-Agent"))
	require.Equal(t, "value", result.Get(name))
}
