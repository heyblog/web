package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type OAuthProvider string

const (
	ProviderGitHub    OAuthProvider = "GITHUB"
	ProviderBlogsClub OAuthProvider = "BLOGSCLUB"
)

func (provider OAuthProvider) slug() string { return strings.ToLower(string(provider)) }

type oauthIdentity struct {
	Subject, Login, Name, Email string
	Profile                     json.RawMessage
	BlogURL                     string
	Verified                    bool
}

type oauthProviderAdapter interface {
	authorizationURL(string) (string, error)
	identity(context.Context, string) (oauthIdentity, error)
}

type OAuthStartOptions struct {
	Mode, Intent, Next, ClaimID string
}

type OAuthStartResult struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
}

type OAuthResult struct {
	Provider  string `json:"provider"`
	State     string `json:"state"`
	Intent    string `json:"intent"`
	Mode      string `json:"mode"`
	Next      string `json:"next"`
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"error_code,omitempty"`
	ClaimID   string `json:"claim_id,omitempty"`
}

// OAuthCompletion keeps server-only failure details separate from the wire result.
// A Failure is reported by the route while Result still completes the popup/redirect flow.
type OAuthCompletion struct {
	Result  OAuthResult
	Cookies []string
	Failure error
}

type OAuthSiteProof struct {
	Subject, BlogURL string
	Verified         bool
}

// OAuthClaims keeps authentication independent of the site-management implementation.
type OAuthClaims interface {
	ValidateOAuthClaim(context.Context, User, string) error
	CompleteOAuthClaim(context.Context, User, string, OAuthSiteProof) error
}

func (service *Service) SetOAuthClaims(claims OAuthClaims) { service.claims = claims }

func (service *Service) provider(provider OAuthProvider) (oauthProviderAdapter, error) {
	switch provider {
	case ProviderGitHub:
		return githubAdapter{service}, nil
	case ProviderBlogsClub:
		return blogsClubAdapter{service}, nil
	default:
		return nil, newAuthError("invalid_oauth_provider", http.StatusNotFound, "OAuth provider was not found")
	}
}

func safeOAuthNext(value string) string {
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, "\\") || strings.ContainsFunc(value, func(r rune) bool { return r < 32 || r == 127 }) {
		return "/dashboard"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/dashboard"
	}
	return value
}

func oauthDisplayName(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 128 {
		value = string(runes[:128])
	}
	return strings.TrimSpace(value)
}

func oauthUsername(login, providerID string) string {
	normalized := strings.ToLower(login)
	var builder strings.Builder
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	value := strings.Trim(builder.String(), "_")
	if len(value) < 3 {
		value = "user"
	}
	if len(value) > 23 {
		value = value[:23]
	}
	suffix := providerID
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return value + "_" + suffix
}
