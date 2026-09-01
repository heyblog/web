package site

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/idna"
)

var (
	ErrInvalidSiteAddress = errors.New("invalid site address")
	ErrInvalidCustomID    = errors.New("invalid custom ID")
)

type Address struct {
	Scheme         string
	NormalizedHost string
	BasePath       string
}

type Location struct {
	Type        string
	URLRef      string
	ExternalURL string
	URLKey      string
}

type FriendTarget struct {
	URL            string
	NormalizedHost string
}

// HomepageURL reconstructs the canonical public homepage from stored address parts.
func (address Address) HomepageURL() (string, error) {
	if address.Scheme != "http" && address.Scheme != "https" {
		return "", fmt.Errorf("%w: scheme must be http or https", ErrInvalidSiteAddress)
	}
	host, err := normalizeHost(address.NormalizedHost)
	if err != nil {
		return "", err
	}
	escapedPath, err := canonicalEscapedPath(address.BasePath)
	if err != nil {
		return "", fmt.Errorf("%w: normalize base path: %w", ErrInvalidSiteAddress, err)
	}
	parsed := &url.URL{Scheme: address.Scheme, Host: host}
	if err := setEscapedPath(parsed, escapedPath); err != nil {
		return "", fmt.Errorf("%w: build base path: %w", ErrInvalidSiteAddress, err)
	}
	return parsed.String(), nil
}

// LocationURL reconstructs a stored relative or external site resource URL.
func (address Address) LocationURL(location Location) (string, error) {
	if location.Type == "EXTERNAL" {
		normalized, err := NormalizeLocation(location.ExternalURL, address, false)
		if err != nil || normalized.Type != "EXTERNAL" {
			return "", fmt.Errorf("%w: invalid external location", ErrInvalidSiteAddress)
		}
		return normalized.ExternalURL, nil
	}
	if location.Type != "RELATIVE" {
		return "", fmt.Errorf("%w: unsupported location type", ErrInvalidSiteAddress)
	}
	normalized, err := NormalizeLocation(location.URLRef, address, false)
	if err != nil || normalized.Type != "RELATIVE" {
		return "", fmt.Errorf("%w: invalid relative location", ErrInvalidSiteAddress)
	}
	homepage, err := url.Parse(address.Scheme + "://" + address.NormalizedHost)
	if err != nil {
		return "", fmt.Errorf("%w: build site origin: %w", ErrInvalidSiteAddress, err)
	}
	reference, err := url.Parse(normalized.URLRef)
	if err != nil {
		return "", fmt.Errorf("%w: build location reference: %w", ErrInvalidSiteAddress, err)
	}
	return homepage.ResolveReference(reference).String(), nil
}

// NormalizeAddress separates a canonical site URL into scheme, IDNA hostname,
// and root-relative installation path.
func NormalizeAddress(raw string) (Address, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Address{}, fmt.Errorf("%w: parse URL: %w", ErrInvalidSiteAddress, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Address{}, fmt.Errorf("%w: scheme must be http or https", ErrInvalidSiteAddress)
	}
	if parsed.User != nil || parsed.Port() != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Address{}, fmt.Errorf("%w: credentials, port, query, and fragment are not allowed", ErrInvalidSiteAddress)
	}

	host, err := normalizeHost(parsed.Hostname())
	if err != nil {
		return Address{}, err
	}

	basePath, err := canonicalEscapedPath(parsed.EscapedPath())
	if err != nil {
		return Address{}, fmt.Errorf("%w: normalize base path: %w", ErrInvalidSiteAddress, err)
	}
	return Address{
		Scheme:         parsed.Scheme,
		NormalizedHost: host,
		BasePath:       basePath,
	}, nil
}

// CanonicalURL reconstructs the normalized registration address.
func (address Address) CanonicalURL() string {
	return address.Scheme + "://" + address.NormalizedHost + address.BasePath
}

// NormalizeLocation converts same-host URLs to a root-relative reference and
// keeps cross-host URLs absolute. Known marketing parameters can be removed
// when the location is used as an article identity.
func NormalizeLocation(raw string, siteAddress Address, removeTracking bool) (Location, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil {
		return Location{}, fmt.Errorf("%w: parse resource URL: %w", ErrInvalidSiteAddress, err)
	}
	if parsed.Fragment != "" {
		parsed.Fragment = ""
	}
	if parsed.User != nil {
		return Location{}, fmt.Errorf("%w: URL credentials are not allowed", ErrInvalidSiteAddress)
	}
	if removeTracking {
		removeTrackingParameters(parsed)
	}

	if !parsed.IsAbs() {
		if parsed.Host != "" {
			return Location{}, fmt.Errorf("%w: relative location must be root-relative", ErrInvalidSiteAddress)
		}
		urlRef, resolveErr := resolveLocationReference(parsed, siteAddress.BasePath)
		if resolveErr != nil {
			return Location{}, resolveErr
		}
		return Location{Type: "RELATIVE", URLRef: urlRef, URLKey: urlRef}, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Location{}, fmt.Errorf("%w: resource scheme must be http or https", ErrInvalidSiteAddress)
	}

	host, err := normalizeHost(parsed.Hostname())
	if err != nil {
		return Location{}, err
	}
	if host == siteAddress.NormalizedHost && normalizedURLHost(host, parsed.Port(), parsed.Scheme) == host {
		urlRef, buildErr := buildURLRef(parsed)
		if buildErr != nil {
			return Location{}, fmt.Errorf("%w: normalize resource path: %w", ErrInvalidSiteAddress, buildErr)
		}
		return Location{Type: "RELATIVE", URLRef: urlRef, URLKey: urlRef}, nil
	}

	parsed.Host = normalizedURLHost(host, parsed.Port(), parsed.Scheme)
	if err := setEscapedPath(parsed, cleanRootRelativePath(parsed.EscapedPath())); err != nil {
		return Location{}, fmt.Errorf("%w: normalize resource path: %w", ErrInvalidSiteAddress, err)
	}
	external := parsed.String()
	return Location{Type: "EXTERNAL", ExternalURL: external, URLKey: external}, nil
}

func resolveLocationReference(reference *url.URL, basePath string) (string, error) {
	if strings.HasPrefix(reference.EscapedPath(), "/") {
		urlRef, err := buildURLRef(reference)
		if err != nil {
			return "", fmt.Errorf("%w: normalize resource path: %w", ErrInvalidSiteAddress, err)
		}
		return urlRef, nil
	}

	normalizedBasePath := cleanRootRelativePath(basePath)
	baseDirectory := strings.TrimSuffix(normalizedBasePath, "/") + "/"
	base, err := url.Parse(baseDirectory)
	if err != nil {
		return "", fmt.Errorf("%w: parse site base path: %w", ErrInvalidSiteAddress, err)
	}
	resolved := base.ResolveReference(reference)
	resolvedPath := cleanRootRelativePath(resolved.EscapedPath())
	if !pathWithinBase(resolvedPath, normalizedBasePath) {
		return "", fmt.Errorf("%w: relative location escapes the site base path", ErrInvalidSiteAddress)
	}

	decodedBasePath, err := url.PathUnescape(normalizedBasePath)
	if err != nil {
		return "", fmt.Errorf("%w: decode site base path: %w", ErrInvalidSiteAddress, err)
	}
	semanticBasePath := cleanRootRelativePath(decodedBasePath)
	semanticBase := &url.URL{Path: strings.TrimSuffix(semanticBasePath, "/") + "/"}
	semanticReference := *reference
	semanticReference.RawPath = ""
	semanticResolvedPath := cleanRootRelativePath(semanticBase.ResolveReference(&semanticReference).Path)
	if !pathWithinBase(semanticResolvedPath, semanticBasePath) {
		return "", fmt.Errorf("%w: relative location escapes the site base path", ErrInvalidSiteAddress)
	}

	urlRef, err := buildURLRef(resolved)
	if err != nil {
		return "", fmt.Errorf("%w: normalize resource path: %w", ErrInvalidSiteAddress, err)
	}
	return urlRef, nil
}

func pathWithinBase(candidate, base string) bool {
	return base == "/" || candidate == base || strings.HasPrefix(candidate, base+"/")
}

// ValidateCustomID checks the case-sensitive custom route grammar before persistence.
func ValidateCustomID(value string) error {
	if len(value) < 3 || len(value) > 32 || !isASCIIAlphanumeric(value[0]) || !isASCIIAlphanumeric(value[len(value)-1]) {
		return ErrInvalidCustomID
	}
	previousSeparator := false
	for index := range value {
		character := value[index]
		if isASCIIAlphanumeric(character) {
			previousSeparator = false
			continue
		}
		separator := character == '-' || character == '_'
		if !separator || previousSeparator {
			return ErrInvalidCustomID
		}
		previousSeparator = true
	}
	return nil
}

func isASCIIAlphanumeric(character byte) bool {
	return character >= '0' && character <= '9' ||
		character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z'
}

func normalizeHost(raw string) (string, error) {
	trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if trimmed == "" || net.ParseIP(trimmed) != nil {
		return "", fmt.Errorf("%w: hostname is empty or an IP address", ErrInvalidSiteAddress)
	}
	ascii, err := idna.Lookup.ToASCII(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: normalize IDNA hostname: %w", ErrInvalidSiteAddress, err)
	}
	if len(ascii) > 253 {
		return "", fmt.Errorf("%w: hostname is too long", ErrInvalidSiteAddress)
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("%w: invalid hostname label", ErrInvalidSiteAddress)
		}
		for _, character := range label {
			if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
				continue
			}
			return "", fmt.Errorf("%w: invalid hostname character", ErrInvalidSiteAddress)
		}
	}
	return ascii, nil
}

func cleanRootRelativePath(raw string) string {
	if raw == "" {
		return "/"
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(raw, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func buildURLRef(parsed *url.URL) (string, error) {
	urlRef, err := canonicalEscapedPath(parsed.EscapedPath())
	if err != nil {
		return "", err
	}
	if parsed.RawQuery != "" {
		urlRef += "?" + parsed.RawQuery
	}
	return urlRef, nil
}

func canonicalEscapedPath(raw string) (string, error) {
	decodedPath, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	for _, segment := range strings.Split(decodedPath, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("path must not contain dot segments")
		}
	}
	return cleanRootRelativePath(raw), nil
}

func setEscapedPath(parsed *url.URL, escapedPath string) error {
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return err
	}
	parsed.Path = decodedPath
	parsed.RawPath = escapedPath
	return nil
}

func removeTrackingParameters(parsed *url.URL) {
	query := parsed.Query()
	for key := range query {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "utm_") || lowerKey == "fbclid" || lowerKey == "gclid" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
}

func normalizedURLHost(host, port, scheme string) string {
	if port == "" || scheme == "http" && port == "80" || scheme == "https" && port == "443" {
		return host
	}
	return net.JoinHostPort(host, port)
}
