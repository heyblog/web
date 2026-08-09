package site

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestGenerateShortIDUsesBase62AndRejectsBiasedBytes(t *testing.T) {
	t.Parallel()

	input := append(bytes.Repeat([]byte{248}, 32), bytes.Repeat([]byte{61}, 32)...)
	got, err := generateShortID(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("generateShortID() error = %v", err)
	}
	if got != strings.Repeat("z", ShortIDLength) {
		t.Fatalf("generateShortID() = %q, want nine z characters", got)
	}
}

func TestGenerateShortIDReportsEntropyFailure(t *testing.T) {
	t.Parallel()

	_, err := generateShortID(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("generateShortID() error = nil, want entropy error")
	}
}

func TestValidateCustomIDAcceptsCaseSensitiveRouteAlphabet(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"My_Blog-2026", "abc", "A1_b-2"} {
		if err := ValidateCustomID(value); err != nil {
			t.Errorf("ValidateCustomID(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"ab", "_blog", "blog-", "my--blog", "my_-blog", "blog space"} {
		if err := ValidateCustomID(value); !errors.Is(err, ErrInvalidCustomID) {
			t.Errorf("ValidateCustomID(%q) error = %v, want ErrInvalidCustomID", value, err)
		}
	}
}

func TestNormalizeAddress(t *testing.T) {
	t.Parallel()

	got, err := NormalizeAddress("https://\u4f8b\u5b50.\u4e2d\u56fd/blog/")
	if err != nil {
		t.Fatalf("NormalizeAddress() error = %v", err)
	}
	if got.Scheme != "https" || got.NormalizedHost != "xn--fsqu00a.xn--fiqs8s" || got.BasePath != "/blog" {
		t.Fatalf("NormalizeAddress() = %#v", got)
	}
}

func TestNormalizeAddressRejectsIP(t *testing.T) {
	t.Parallel()

	_, err := NormalizeAddress("https://127.0.0.1/")
	if !errors.Is(err, ErrInvalidSiteAddress) {
		t.Fatalf("NormalizeAddress() error = %v, want ErrInvalidSiteAddress", err)
	}
}

func TestNormalizeAddressRejectsPort(t *testing.T) {
	t.Parallel()

	_, err := NormalizeAddress("https://example.com:8443/blog")
	if !errors.Is(err, ErrInvalidSiteAddress) {
		t.Fatalf("NormalizeAddress() error = %v, want ErrInvalidSiteAddress", err)
	}
}

func TestNormalizeAddressRejectsEncodedDotSegments(t *testing.T) {
	t.Parallel()

	_, err := NormalizeAddress("https://example.com/%2e%2e/admin")
	if !errors.Is(err, ErrInvalidSiteAddress) {
		t.Fatalf("NormalizeAddress() error = %v, want ErrInvalidSiteAddress", err)
	}
}

func TestNormalizeLocation(t *testing.T) {
	t.Parallel()

	got, err := NormalizeLocation(
		"https://blog.example.com/posts/1?utm_source=test&lang=zh#title",
		Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: "/blog"},
		true,
	)
	if err != nil {
		t.Fatalf("NormalizeLocation() error = %v", err)
	}
	if got.Type != "RELATIVE" || got.URLRef != "/posts/1?lang=zh" || got.URLKey != got.URLRef {
		t.Fatalf("NormalizeLocation() = %#v", got)
	}
}

func TestNormalizeLocationKeepsCrossHostURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeLocation(
		"https://feeds.example.net/blog.xml",
		Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: "/blog"},
		false,
	)
	if err != nil {
		t.Fatalf("NormalizeLocation() error = %v", err)
	}
	if got.Type != "EXTERNAL" || got.ExternalURL != "https://feeds.example.net/blog.xml" {
		t.Fatalf("NormalizeLocation() = %#v", got)
	}
}

func TestNormalizeLocationPreservesEscapedExternalPath(t *testing.T) {
	t.Parallel()

	got, err := NormalizeLocation(
		"https://feeds.example.net/a%2Fb/feed%20file.xml",
		Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: "/blog"},
		false,
	)
	if err != nil {
		t.Fatalf("NormalizeLocation() error = %v", err)
	}
	if got.ExternalURL != "https://feeds.example.net/a%2Fb/feed%20file.xml" {
		t.Fatalf("ExternalURL = %q, want escaped path preserved", got.ExternalURL)
	}
}

func TestNormalizeLocationKeepsNonDefaultPortExternal(t *testing.T) {
	t.Parallel()

	got, err := NormalizeLocation(
		"https://blog.example.com:8443/feed.xml",
		Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: "/blog"},
		false,
	)
	if err != nil {
		t.Fatalf("NormalizeLocation() error = %v", err)
	}
	if got.Type != "EXTERNAL" || got.ExternalURL != "https://blog.example.com:8443/feed.xml" {
		t.Fatalf("NormalizeLocation() = %#v", got)
	}
}

func TestNormalizeLocationResolvesRelativePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		basePath string
		raw      string
		want     string
	}{
		{name: "installation relative", basePath: "/blog", raw: "feed.xml?lang=zh", want: "/blog/feed.xml?lang=zh"},
		{name: "host root relative", basePath: "/blog", raw: "/feed.xml", want: "/feed.xml"},
		{name: "root installation", basePath: "/", raw: "feed.xml", want: "/feed.xml"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeLocation(
				test.raw,
				Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: test.basePath},
				false,
			)
			if err != nil {
				t.Fatalf("NormalizeLocation() error = %v", err)
			}
			if got.Type != "RELATIVE" || got.URLRef != test.want || got.URLKey != test.want {
				t.Fatalf("NormalizeLocation() = %#v, want URL reference %q", got, test.want)
			}
		})
	}
}

func TestNormalizeLocationResolvesAgainstEscapedBasePath(t *testing.T) {
	t.Parallel()

	address, err := NormalizeAddress("https://example.com/%E5%8D%9A%E5%AE%A2")
	if err != nil {
		t.Fatalf("NormalizeAddress() error = %v", err)
	}
	got, err := NormalizeLocation("feed%20file.xml", address, false)
	if err != nil {
		t.Fatalf("NormalizeLocation() error = %v", err)
	}
	if got.URLRef != "/%E5%8D%9A%E5%AE%A2/feed%20file.xml" {
		t.Fatalf("URLRef = %q, want escaped base and resource path preserved", got.URLRef)
	}
}

func TestNormalizeLocationRejectsPathEscapingBasePath(t *testing.T) {
	t.Parallel()

	for _, location := range []string{"../feed.xml", "%2e%2e/feed.xml", ".%2e/feed.xml"} {
		_, err := NormalizeLocation(
			location,
			Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: "/blog"},
			false,
		)
		if !errors.Is(err, ErrInvalidSiteAddress) {
			t.Errorf("NormalizeLocation(%q) error = %v, want ErrInvalidSiteAddress", location, err)
		}
	}
}

func TestNormalizeLocationRejectsEncodedDotSegmentsFromCanonicalInputs(t *testing.T) {
	t.Parallel()

	address := Address{Scheme: "https", NormalizedHost: "blog.example.com", BasePath: "/blog"}
	for _, location := range []string{
		"/%2e%2e/admin",
		"https://blog.example.com/blog/%2e%2e/admin",
	} {
		_, err := NormalizeLocation(location, address, false)
		if !errors.Is(err, ErrInvalidSiteAddress) {
			t.Errorf("NormalizeLocation(%q) error = %v, want ErrInvalidSiteAddress", location, err)
		}
	}
}

func TestNormalizeFriendTargetKeepsAbsoluteURLAndNormalizedHost(t *testing.T) {
	t.Parallel()

	got, err := NormalizeFriendTarget("https://例子.中国/friends?from=a#directory")
	if err != nil {
		t.Fatalf("NormalizeFriendTarget() error = %v", err)
	}
	if got.NormalizedHost != "xn--fsqu00a.xn--fiqs8s" {
		t.Fatalf("NormalizeFriendTarget().NormalizedHost = %q", got.NormalizedHost)
	}
	if got.URL != "https://xn--fsqu00a.xn--fiqs8s/friends?from=a" {
		t.Fatalf("NormalizeFriendTarget().URL = %q", got.URL)
	}
}

func TestNormalizeFriendTargetPreservesEscapedPath(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"escaped delimiters": "https://example.com/a%2Fb/feed%20file.xml",
		"unicode":            "https://example.com/%E5%8D%9A%E5%AE%A2",
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := NormalizeFriendTarget(target)
			if err != nil {
				t.Fatalf("NormalizeFriendTarget() error = %v", err)
			}
			if got.URL != target {
				t.Fatalf("URL = %q, want %q", got.URL, target)
			}
		})
	}
}

func TestNormalizeFriendTargetRejectsRelativeAndPortURLs(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"/friends", "https://example.com:8443/friends"} {
		if _, err := NormalizeFriendTarget(value); !errors.Is(err, ErrInvalidFriendTarget) {
			t.Errorf("NormalizeFriendTarget(%q) error = %v, want ErrInvalidFriendTarget", value, err)
		}
	}
}
