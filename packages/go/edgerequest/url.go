package edgerequest

import (
	"errors"
	"net/netip"
	"net/url"
	"strings"
)

func NormalizeURL(raw string) (string, error) {
	u, err := canonicalTarget(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// canonicalTarget normalizes equivalent HTTP URL spellings before DNS checks and
// signing. It does not resolve ambiguous paths; Edge also rejects any remaining
// spelling that native Fetch would rewrite.
func canonicalTarget(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || strings.Contains(raw, `\`) || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Port() != "" && parsed.Port() != "80" && parsed.Port() != "443") {
		return nil, &Error{Kind: Security, Cause: errors.New("invalid target URL")}
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, &Error{Kind: Security, Cause: errors.New("ambiguous target path")}
		}
	}
	host, port := strings.ToLower(parsed.Hostname()), parsed.Port()
	if ip, err := netip.ParseAddr(host); err == nil {
		host = ip.String()
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	defaultPort := parsed.Scheme == "http" && port == "80" || parsed.Scheme == "https" && port == "443"
	if port != "" && !defaultPort {
		host += ":" + port
	}
	parsed.Host = host
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.RawQuery = escapeSpecialQuery(parsed.RawQuery)
	if len(parsed.String()) > 8192 {
		return nil, &Error{Kind: Security, Cause: errors.New("target URL exceeds edge limit")}
	}
	return parsed, nil
}

// HTTP(S) uses the WHATWG special-query percent-encode set. Preserve existing
// escapes, parameter order and '+' so normalization cannot change query values.
func escapeSpecialQuery(query string) string {
	const hex = "0123456789ABCDEF"
	var result strings.Builder
	result.Grow(len(query))
	for i := range len(query) {
		b := query[i]
		if b <= 0x20 || b > 0x7e || strings.ContainsRune(`"#<>'`, rune(b)) {
			result.WriteByte('%')
			result.WriteByte(hex[b>>4])
			result.WriteByte(hex[b&15])
		} else {
			result.WriteByte(b)
		}
	}
	return result.String()
}
