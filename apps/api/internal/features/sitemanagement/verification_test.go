package sitemanagement

import (
	"context"
	"errors"
	"heyblog-api/internal/platform/apperror"
	"net"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

const testToken = "0123456789012345678901234567890123456789012"

type testResolver struct{ records []string }

func (r testResolver) LookupTXT(context.Context, string) ([]string, error) { return r.records, nil }
func TestVerifyDNS_when_RecordMatchesDigest(t *testing.T) {
	t.Parallel()
	// Given a TXT proof and its stored digest.
	verifier := NewDNSVerifier(testResolver{records: []string{proofPrefix + testToken}})
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
func TestHTTPProofParsing(t *testing.T) {
	for _, method := range []Method{Meta, File} {
		t.Run(string(method), func(t *testing.T) {
			claim := Claim{Method: method, TokenHash: digest(testToken)}
			content := testToken + "\n"
			limit := 512
			if method == Meta {
				content = `<html><head><meta name="heyblog-site-verification" content="` + testToken + `"></head></html>`
				limit = 1 << 20
			}
			require.True(t, CheckHTTPProof(claim, 200, []byte(content)))
			require.False(t, CheckHTTPProof(claim, 200, []byte(strings.ReplaceAll(content, testToken, strings.Repeat("x", 43)))))
			require.False(t, CheckHTTPProof(claim, 200, []byte(content+strings.Repeat(" ", limit))))
			require.False(t, CheckHTTPProof(claim, 404, []byte(content)))
		})
	}
}

type deadlineResolver struct{}

func (deadlineResolver) LookupTXT(ctx context.Context, _ string) ([]string, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestAutomaticVerificationSharesEightSecondDeadline(t *testing.T) {
	for _, method := range []Method{DNS} {
		t.Run(string(method), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				verifier := NewDNSVerifier(deadlineResolver{})
				err := verifier.Verify(t.Context(), Claim{Address: "https://verification.example/", Method: method, TokenHash: digest(testToken)})
				require.Error(t, err)
				require.Equal(t, 8*time.Second, time.Since(started))
				var application *apperror.Error
				require.ErrorAs(t, verificationError(err), &application)
				require.Equal(t, "verification_timeout", application.Code())
			})
		})
	}
}

func TestVerificationFailuresPreserveSafeCategories(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
		code  string
	}{
		{"mismatch", errProof, "proof_not_found"},
		{"missing DNS", &net.DNSError{IsNotFound: true, Name: "private diagnostic hostname"}, "proof_not_found"},
		{"network", errors.New("private connection diagnostic"), "verification_network"},
		{"timeout", context.DeadlineExceeded, "verification_timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var application *apperror.Error
			require.ErrorAs(t, routeError(verificationError(test.cause)), &application)
			require.Equal(t, test.code, application.Code())
			require.NotContains(t, application.PublicDetail(), "private")
			require.ErrorIs(t, application, test.cause)
		})
	}
}
