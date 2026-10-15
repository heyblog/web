package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/platform/apperror"
)

const oauthStateTTL = 10 * time.Minute

type oauthState struct {
	OAuthStartOptions
	Provider    OAuthProvider
	ActorID     string
	AuthVersion int32
	ExpiresAt   time.Time
}

func oauthStateKey(provider OAuthProvider, token string) string {
	return "heyblog:auth:oauth:" + provider.slug() + ":" + token
}

func (service *Service) OAuthStart(ctx context.Context, request *http.Request, provider OAuthProvider, options OAuthStartOptions) (OAuthStartResult, error) {
	adapter, err := service.provider(provider)
	if err != nil {
		return OAuthStartResult{}, err
	}
	if options.Mode == "" {
		options.Mode = "redirect"
	}
	if options.Intent == "" {
		options.Intent = "login"
	}
	if (options.Mode != "popup" && options.Mode != "redirect") || (options.Intent != "login" && options.Intent != "bind" && options.Intent != "site-claim") {
		return OAuthStartResult{}, newAuthError("invalid_oauth_intent", 422, "OAuth mode or intent is invalid")
	}
	state := oauthState{OAuthStartOptions: options, Provider: provider, ExpiresAt: time.Now().Add(oauthStateTTL)}
	state.Next = safeOAuthNext(options.Next)
	if options.Intent != "login" {
		actor, currentErr := service.Current(ctx, request)
		if currentErr != nil {
			return OAuthStartResult{}, currentErr
		}
		state.ActorID, state.AuthVersion = actor.ID, actor.AuthVersion
		if options.Intent == "site-claim" {
			if provider != ProviderBlogsClub || service.claims == nil {
				return OAuthStartResult{}, newAuthError("invalid_oauth_intent", 422, "This provider cannot verify sites")
			}
			if err := service.claims.ValidateOAuthClaim(ctx, actor, options.ClaimID); err != nil {
				return OAuthStartResult{}, err
			}
			state.Next = "/dashboard/claims/" + options.ClaimID
		}
	}
	token, err := randomToken(24)
	if err != nil {
		return OAuthStartResult{}, err
	}
	target, err := adapter.authorizationURL(token)
	if err != nil {
		return OAuthStartResult{}, err
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return OAuthStartResult{}, err
	}
	if err := service.redis.Set(ctx, oauthStateKey(provider, token), encoded, oauthStateTTL).Err(); err != nil {
		return OAuthStartResult{}, repositoryError("store OAuth state", err)
	}
	return OAuthStartResult{AuthorizationURL: target, State: token}, nil
}

func (service *Service) OAuthCallback(ctx context.Context, request *http.Request, provider OAuthProvider, code, token, cookie, denied string) (OAuthResult, []string, error) {
	if token == "" || cookie == "" || token != cookie {
		return OAuthResult{}, nil, newAuthError("invalid_oauth_state", 400, "OAuth state is invalid")
	}
	value, err := service.redis.GetDel(ctx, oauthStateKey(provider, token)).Result()
	if errors.Is(err, redis.Nil) {
		return OAuthResult{}, nil, newAuthError("invalid_oauth_state", 400, "OAuth state is invalid")
	}
	if err != nil {
		return OAuthResult{}, nil, repositoryError("consume OAuth state", err)
	}
	var state oauthState
	if err := json.Unmarshal([]byte(value), &state); err != nil || state.Provider != provider || !time.Now().Before(state.ExpiresAt) {
		return OAuthResult{}, nil, newAuthError("invalid_oauth_state", 400, "OAuth state is invalid")
	}
	result := OAuthResult{Provider: provider.slug(), State: token, Intent: state.Intent, Mode: state.Mode, Next: state.Next, ClaimID: state.ClaimID, Outcome: "error"}
	if denied != "" {
		result.ErrorCode = "oauth_denied"
		return result, nil, nil
	}
	if code == "" {
		result.ErrorCode = "oauth_code_required"
		return result, nil, nil
	}
	cookies, err := service.completeOAuth(ctx, request, provider, code, state, &result)
	if err != nil {
		result.ErrorCode = oauthErrorCode(err)
		return result, cookies, nil
	}
	result.Outcome = "success"
	return result, cookies, nil
}

func (service *Service) completeOAuth(ctx context.Context, request *http.Request, provider OAuthProvider, code string, state oauthState, result *OAuthResult) ([]string, error) {
	var actor User
	if state.Intent != "login" {
		current, err := service.Current(ctx, request)
		if err != nil {
			return nil, err
		}
		if current.ID != state.ActorID || current.AuthVersion != state.AuthVersion {
			return nil, newAuthError("oauth_actor_changed", 409, "The account changed; restart authorization")
		}
		actor = current
	}
	adapter, err := service.provider(provider)
	if err != nil {
		return nil, err
	}
	identity, err := adapter.identity(ctx, code)
	if err != nil {
		return nil, err
	}
	if !time.Now().Before(state.ExpiresAt) {
		return nil, newAuthError("invalid_oauth_state", 400, "OAuth authorization expired")
	}
	if state.Intent != "login" {
		if err := service.repo.bindOAuth(ctx, actor, provider, identity); err != nil {
			return nil, err
		}
		if state.Intent == "site-claim" {
			if service.claims == nil {
				return nil, newAuthError("oauth_unavailable", 503, "Site verification is unavailable")
			}
			return nil, service.claims.CompleteOAuthClaim(ctx, actor, state.ClaimID, OAuthSiteProof{Subject: identity.Subject, BlogURL: identity.BlogURL, Verified: identity.Verified})
		}
		return nil, nil
	}
	record, err := service.repo.loginOAuth(ctx, provider, identity)
	if err != nil {
		return nil, err
	}
	access, refresh, err := service.issueSession(ctx, record)
	if err != nil {
		return nil, err
	}
	if record.ProfileConfirmationStatus == "PENDING" {
		result.Next = "/complete-profile?next=" + url.QueryEscape(result.Next)
	}
	return []string{access, refresh}, nil
}

func oauthErrorCode(err error) string {
	var authError *AuthError
	if errors.As(err, &authError) {
		return authError.Code
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return applicationError.Code()
	}
	return "oauth_failed"
}
