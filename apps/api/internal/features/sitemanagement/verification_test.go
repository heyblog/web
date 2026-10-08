package sitemanagement

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testToken = "0123456789012345678901234567890123456789012"

func TestVerificationTransport_when_FirstPublicAddressIsUnreachable(t *testing.T) {
	// Given two public DNS addresses and an unreachable first address.
	var attempted []string
	client, server := net.Pipe()
	defer func() { _ = client.Close(); _ = server.Close() }()
	transport := verificationTransport(testResolver{ips: []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111"), netip.MustParseAddr("1.1.1.1")}}, func(_ context.Context, _ string, address string) (net.Conn, error) {
		attempted = append(attempted, address)
		if len(attempted) == 1 {
			return nil, errors.New("unreachable")
		}
		return client, nil
	})
	// When the pinned connection is established.
	connection, err := transport.DialContext(t.Context(), "tcp", "example.com:443")
	// Then the second public address succeeds without re-resolving the hostname.
	require.NoError(t, err)
	require.Equal(t, client, connection)
	require.Equal(t, []string{"[2606:4700:4700::1111]:443", "1.1.1.1:443"}, attempted)
}

func TestVerificationTransport_when_AnyAddressIsPrivate(t *testing.T) {
	// Given mixed public/private DNS results.
	transport := verificationTransport(testResolver{ips: []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}}, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("unsafe result reached the dialer")
		return nil, errors.New("unexpected dial")
	})
	// When the whole result is checked, no address may be dialed.
	_, err := transport.DialContext(t.Context(), "tcp", "example.com:443")
	require.ErrorContains(t, err, "not public")
}

type testResolver struct {
	ips     []netip.Addr
	records []string
}

func (r testResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.ips, nil
}
func (r testResolver) LookupTXT(context.Context, string) ([]string, error) { return r.records, nil }
func TestVerifyDNS_when_RecordMatchesDigest(t *testing.T) {
	t.Parallel()
	// Given a TXT proof and its stored digest.
	verifier := newNetworkVerifier(testResolver{records: []string{proofPrefix + testToken}})
	// When the challenge is checked.
	err := verifier.Verify(t.Context(), Claim{Address: "https://example.com/", Method: DNS, TokenHash: digest(testToken)})
	// Then the TXT proof is accepted.
	require.NoError(t, err)
}
func TestVerifyMeta_when_ProofIsInsideBody(t *testing.T) {
	t.Parallel()
	// Given a meta tag placed outside the document head.
	content := `<html><head></head><body><meta name="heyblog-site-verification" content="` + testToken + `"></body></html>`
	// When the proof is parsed as HTML.
	err := verifyMeta(content, digest(testToken))
	// Then a body tag cannot prove control.
	require.ErrorIs(t, err, errProof)
}
func TestVerifyHTTP_when_PublicProofMatches(t *testing.T) {
	for _, method := range []Method{Meta, File} {
		t.Run(string(method), func(t *testing.T) {
			// Given a real HTTP server publishing the expected proof.
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if method == File {
					require.Equal(t, "/blog/.well-known/heyblog-site-verification.txt", r.URL.Path)
					_, err := fmt.Fprint(w, testToken+"\n")
					require.NoError(t, err)
					return
				}
				_, err := fmt.Fprintf(w, `<html><head><meta content="%s" name="heyblog-site-verification"></head><body></body></html>`, testToken)
				require.NoError(t, err)
			}))
			defer server.Close()
			verifier := &NetworkVerifier{client: server.Client()}
			// When the network proof is inspected.
			err := verifier.Verify(t.Context(), Claim{Address: server.URL + "/blog/", Method: method, TokenHash: digest(testToken)})
			// Then the exact file or head proof is accepted.
			require.NoError(t, err)
		})
	}
}
func TestVerifyHTTP_when_FileExceedsLimit(t *testing.T) {
	// Given a file response larger than the proof limit.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, testToken+strings.Repeat(" ", 600))
		require.NoError(t, err)
	}))
	defer server.Close()
	verifier := &NetworkVerifier{client: server.Client()}
	// When the file is checked.
	err := verifier.Verify(t.Context(), Claim{Address: server.URL, Method: File, TokenHash: digest(testToken)})
	// Then oversized responses are rejected.
	require.ErrorIs(t, err, errProof)
}
func TestVerifyHTTP_when_DNSResolvesToPrivateAddress(t *testing.T) {
	t.Parallel()
	// Given DNS resolution which would reach an internal resource.
	verifier := newNetworkVerifier(testResolver{ips: []netip.Addr{netip.MustParseAddr("127.0.0.1")}})
	// When a seemingly public hostname is verified.
	err := verifier.Verify(t.Context(), Claim{Address: "https://example.com/", Method: Meta, TokenHash: digest(testToken)})
	// Then it is refused before any internal connection.
	require.ErrorContains(t, err, "not public")
}
func TestPublicIP_when_AddressIsReserved(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "198.18.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "2001:db8::1", "2002:7f00:1::"} {
		t.Run(raw, func(t *testing.T) { require.False(t, publicIP(netip.MustParseAddr(raw))) })
	}
	require.True(t, publicIP(netip.MustParseAddr("1.1.1.1")))
}
func TestRedirect_when_TargetChangesOrigin(t *testing.T) {
	t.Parallel()
	// Given a client whose source proof is bound to one origin.
	verifier := NewNetworkVerifier()
	source, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/", nil)
	require.NoError(t, err)
	target, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://other.example/", nil)
	require.NoError(t, err)
	// When the upstream redirects to another site.
	err = verifier.client.CheckRedirect(target, []*http.Request{source})
	// Then another site's proof cannot verify the requested site.
	require.Error(t, err)
}
