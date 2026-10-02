package site

import (
	"fmt"
	"net/url"
	"strings"
)

type Location struct {
	Type        string
	URLRef      string
	ExternalURL string
	URLKey      string
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

// NormalizeLocation normalizes resource references and optionally removes tracking parameters.
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
