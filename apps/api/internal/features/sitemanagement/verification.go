package sitemanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const proofPrefix = "heyblog-site-verification="

var errProof = errors.New("verification proof was not found")

type Resolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

// DNSVerifier performs the only synchronous ownership check. HTTP proofs are
// executed by a worker so the API never connects to an Edge executor directly.
type DNSVerifier struct{ resolver Resolver }

func NewDNSVerifier(resolver Resolver) *DNSVerifier { return &DNSVerifier{resolver: resolver} }

func (v *DNSVerifier) Verify(ctx context.Context, claim Claim) error {
	if claim.Method != DNS {
		return errors.New("verification method requires asynchronous check")
	}
	ctx, cancel := context.WithTimeout(ctx, VerificationTimeout)
	defer cancel()
	return verifyDNS(ctx, v.resolver, claim)
}

func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func matches(token, hash string) bool { return len(token) == 43 && digest(token) == hash }
func verifyDNS(ctx context.Context, resolver Resolver, claim Claim) error {
	parsed, err := url.Parse(claim.Address)
	if err != nil {
		return err
	}
	records, err := resolver.LookupTXT(ctx, "_heyblog-verification."+parsed.Hostname())
	if err != nil {
		return err
	}
	for _, record := range records {
		if strings.HasPrefix(record, proofPrefix) && matches(strings.TrimPrefix(record, proofPrefix), claim.TokenHash) {
			return nil
		}
	}
	return errProof
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
