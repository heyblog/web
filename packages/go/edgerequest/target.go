package edgerequest

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// PublicIP rejects private, special-use and transition ranges, including mapped IPv4.
func PublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is6() && (ip.As16()[0]&0xe0) != 0x20 {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func (c *Client) target(ctx context.Context, raw string) (*url.URL, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", &Error{Kind: Cancelled, Cause: err}
	}
	parsed, err := canonicalTarget(raw)
	if err != nil {
		return nil, "", err
	}
	var ips []netip.Addr
	if ip, parseErr := netip.ParseAddr(parsed.Hostname()); parseErr == nil {
		ips = []netip.Addr{ip}
	} else {
		ips, err = c.config.Resolver.LookupNetIP(ctx, "ip", parsed.Hostname())
		if err != nil {
			if ctx.Err() != nil {
				return nil, "", &Error{Kind: Cancelled, Cause: ctx.Err()}
			}
			return nil, "", &Error{Kind: DNS, Cause: err}
		}
	}
	if len(ips) == 0 {
		return nil, "", &Error{Kind: DNS, Cause: errors.New("target has no addresses")}
	}
	for _, ip := range ips {
		if !PublicIP(ip) {
			return nil, "", &Error{Kind: Security, Cause: errors.New("target address is not public")}
		}
	}
	return parsed, ips[0].String(), nil
}

// SafeHeaders restricts task-controlled headers. Service authentication never enters this map.
func SafeHeaders(headers http.Header) (http.Header, error) {
	result := make(http.Header)
	for name, values := range headers {
		key := strings.ToLower(name)
		switch key {
		case "user-agent", "accept", "accept-language", "if-none-match", "if-modified-since", "cache-control", "pragma", "x-heyblog-proof":
		default:
			if !strings.HasPrefix(key, "x-") || strings.HasPrefix(key, "x-forwarded-") || strings.HasPrefix(key, "x-heyblog-") || strings.Contains(key, "authorization") || strings.Contains(key, "token") || strings.Contains(key, "secret") {
				return nil, &Error{Kind: Security, Cause: errors.New("request header is reserved")}
			}
		}
		if len(name) > 128 || !httpguts.ValidHeaderFieldName(name) || len(values) != 1 || len(values[0]) > 8192 || !httpguts.ValidHeaderFieldValue(values[0]) {
			return nil, &Error{Kind: Security, Cause: errors.New("invalid request header")}
		}
		result.Set(name, values[0])
	}
	result.Set("Cache-Control", "no-cache, no-store")
	result.Set("Pragma", "no-cache")
	return result, nil
}
