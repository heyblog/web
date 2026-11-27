package taskcenter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFeed_when_JSONContainsUnstorableText(t *testing.T) {
	for _, item := range []string{
		`"title":"before\u0000after"`,
		`"date_published":"\u0000"`,
		`"id":"\u0000"`,
		"\"title\":\"\xff\"",
	} {
		t.Run(item, func(t *testing.T) {
			// Given a JSON feed containing text that cannot be preserved in PostgreSQL.
			body := []byte(`{"version":"https://jsonfeed.org/version/1.1","items":[{"url":"https://example.com/article",` + item + `}]}`)
			// When parsing the untrusted feed.
			_, err := parseFeed(body)
			// Then it is rejected before persistence.
			require.Error(t, err)
		})
	}
}

func TestStoredFeedHeader_validatesHTTPValidators(t *testing.T) {
	for _, test := range []struct {
		name, header, value, want string
	}{
		{"strong etag", "ETag", `"abc"`, `"abc"`},
		{"weak etag", "ETag", `W/"abc"`, `W/"abc"`},
		{"escaped etag", "ETag", `"a\"b"`, `"a\"b"`},
		{"unquoted etag", "ETag", "abc", ""},
		{"unescaped quote", "ETag", `"a"b"`, ""},
		{"invalid date", "Last-Modified", "not a date", ""},
		{"valid date", "Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT", "Wed, 21 Oct 2015 07:28:00 GMT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, storedFeedHeader(test.header, test.value))
		})
	}
}
