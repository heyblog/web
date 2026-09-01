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

func TestValidateShortIDAcceptsOnlyFixedWidthBase62(t *testing.T) {
	t.Parallel()

	if err := ValidateShortID("A1b2C3d4E"); err != nil {
		t.Fatalf("ValidateShortID() error = %v", err)
	}
	for _, value := range []string{"short", "A1b2C3d4-", "A1b2C3d4_", "A1b2C3d4Ef"} {
		if err := ValidateShortID(value); !errors.Is(err, ErrInvalidShortID) {
			t.Errorf("ValidateShortID(%q) error = %v, want ErrInvalidShortID", value, err)
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

func TestAddressReconstructsHomepageAndStoredLocations(t *testing.T) {
	t.Parallel()

	address := Address{
		Scheme:         "https",
		NormalizedHost: "example.com",
		BasePath:       "/%E5%8D%9A%E5%AE%A2",
	}
	homepage, err := address.HomepageURL()
	if err != nil {
		t.Fatalf("HomepageURL() error = %v", err)
	}
	if homepage != "https://example.com/%E5%8D%9A%E5%AE%A2" {
		t.Fatalf("HomepageURL() = %q", homepage)
	}

	relative, err := address.LocationURL(Location{Type: "RELATIVE", URLRef: "/feed.xml?lang=zh"})
	if err != nil {
		t.Fatalf("LocationURL(relative) error = %v", err)
	}
	if relative != "https://example.com/feed.xml?lang=zh" {
		t.Fatalf("LocationURL(relative) = %q", relative)
	}

	external, err := address.LocationURL(Location{
		Type:        "EXTERNAL",
		ExternalURL: "https://feeds.example.net/a%2Fb.xml",
	})
	if err != nil {
		t.Fatalf("LocationURL(external) error = %v", err)
	}
	if external != "https://feeds.example.net/a%2Fb.xml" {
		t.Fatalf("LocationURL(external) = %q", external)
	}
}

func TestNormalizeAddressBuildsCanonicalURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeAddress("http://example.com/blog")
	if err != nil {
		t.Fatalf("NormalizeAddress() error = %v", err)
	}
	if got.Scheme != "http" || got.NormalizedHost != "example.com" || got.BasePath != "/blog" {
		t.Fatalf("NormalizeAddress() = %#v", got)
	}
	if got.CanonicalURL() != "http://example.com/blog" {
		t.Fatalf("CanonicalURL() = %q, want http://example.com/blog", got.CanonicalURL())
	}
}

func TestNormalizeAddressRejectsNonCanonicalSiteURLParts(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"https://user@example.com/blog",
		"https://example.com/blog?from=directory",
		"https://example.com/blog#friends",
		"https://example.com/%2e%2e/admin",
	} {
		if _, err := NormalizeAddress(value); !errors.Is(err, ErrInvalidSiteAddress) {
			t.Errorf("NormalizeAddress(%q) error = %v, want ErrInvalidSiteAddress", value, err)
		}
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
