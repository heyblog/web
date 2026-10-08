package sitemanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const proofPrefix = "heyblog-site-verification="

var errProof = errors.New("verification proof was not found")

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
	LookupTXT(context.Context, string) ([]string, error)
}
type NetworkVerifier struct {
	resolver Resolver
	client   *http.Client
}

func NewNetworkVerifier() *NetworkVerifier { return newNetworkVerifier(net.DefaultResolver) }
func newNetworkVerifier(resolver Resolver) *NetworkVerifier {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := verificationTransport(resolver, dialer.DialContext)
	return &NetworkVerifier{resolver: resolver, client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
			return errors.New("verification redirect is not allowed")
		}
		return nil
	}}}
}
func verificationTransport(resolver Resolver, dial func(context.Context, string, string) (net.Conn, error)) *http.Transport {
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errProof
		}
		for _, ip := range ips {
			if !publicIP(ip) {
				return nil, errors.New("verification destination is not public")
			}
		}
		var failures []error
		for _, ip := range ips {
			connection, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return connection, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			failures = append(failures, err)
		}
		return nil, errors.Join(failures...)
	}
	return transport
}
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is6() && (ip.As16()[0]&0xe0) != 0x20 {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return false
		}
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func matches(token, hash string) bool { return len(token) == 43 && digest(token) == hash }
func (v *NetworkVerifier) Verify(ctx context.Context, claim Claim) error {
	parsed, err := url.Parse(claim.Address)
	if err != nil {
		return err
	}
	switch claim.Method {
	case DNS:
		records, err := v.resolver.LookupTXT(ctx, "_heyblog-verification."+parsed.Hostname())
		if err != nil {
			return err
		}
		for _, record := range records {
			if strings.HasPrefix(record, proofPrefix) && matches(strings.TrimPrefix(record, proofPrefix), claim.TokenHash) {
				return nil
			}
		}
		return errProof
	case Meta, File:
		if claim.Method == File {
			parsed, err = url.Parse(strings.TrimRight(claim.Address, "/") + "/.well-known/heyblog-site-verification.txt")
			if err != nil {
				return err
			}
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return err
		}
		response, err := v.client.Do(request)
		if err != nil {
			return err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return errProof
		}
		limit := int64(1 << 20)
		if claim.Method == File {
			limit = 512
		}
		content, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
		if err != nil {
			return err
		}
		if int64(len(content)) > limit {
			return errProof
		}
		if claim.Method == File {
			if matches(strings.TrimSpace(string(content)), claim.TokenHash) {
				return nil
			}
			return errProof
		}
		return verifyMeta(string(content), claim.TokenHash)
	case Manual:
		return errors.New("manual verification requires review")
	default:
		return errors.New("unknown verification method")
	}
}
func verifyMeta(content, hash string) error {
	document, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return err
	}
	var walk func(*html.Node, bool) bool
	walk = func(node *html.Node, inHead bool) bool {
		if node.Type == html.ElementNode && node.Data == "head" {
			inHead = true
		}
		if inHead && node.Type == html.ElementNode && node.Data == "meta" {
			var name, content string
			for _, attribute := range node.Attr {
				switch attribute.Key {
				case "name":
					name = attribute.Val
				case "content":
					content = attribute.Val
				}
			}
			if name == "heyblog-site-verification" && matches(content, hash) {
				return true
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if walk(child, inHead) {
				return true
			}
		}
		return false
	}
	if walk(document, false) {
		return nil
	}
	return errProof
}
