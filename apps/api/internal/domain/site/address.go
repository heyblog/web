package site

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrInvalidSiteAddress = errors.New("invalid site address")

type Address struct {
	Scheme         string
	NormalizedHost string
	BasePath       string
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

// NormalizeAddress separates a canonical site URL into scheme, IDNA hostname, and installation path.
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
