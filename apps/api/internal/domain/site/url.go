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
