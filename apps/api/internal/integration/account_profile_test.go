//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"heyblog-api/internal/features/auth"
)

func profileRequest(ctx context.Context, t *testing.T, tokens []string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://api.example.test/api/web/account/profile", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "heyblog_access_token", Value: tokens[0]})
	return request
}

func verifyOAuthProfileChanges(ctx context.Context, t *testing.T, service *auth.Service, user auth.User, tokens []string) []string {
	t.Helper()
	request := profileRequest(ctx, t, tokens)
	if !user.ProfileConfirmationRequired || user.UsernameChangesRemaining != 1 {
		t.Fatalf("new OAuth state: %+v", user)
	}
	if _, err := service.Current(ctx, request); err == nil {
		t.Fatal("business operation accepted pending profile")
	}
	if current, err := service.CurrentSession(ctx, request); err != nil || current.ID != user.ID {
		t.Fatalf("pending session: %v", err)
	}
	confirmed, next, err := service.ConfirmProfile(ctx, request, auth.ProfileConfirmation{Username: "oauth_confirmed", DisplayName: "Confirmed OAuth"})
	if err != nil || confirmed.ProfileConfirmationRequired || confirmed.UsernameChangesRemaining != 1 || len(next) != 2 {
		t.Fatalf("confirmation: %+v / %v", confirmed, err)
	}
	if _, err := service.CurrentSession(ctx, request); err == nil {
		t.Fatal("old session remained usable after initial rename")
	}
	request = profileRequest(ctx, t, next)
	if _, _, err := service.ConfirmProfile(ctx, request, auth.ProfileConfirmation{Username: confirmed.Username, DisplayName: confirmed.DisplayName}); err != nil {
		t.Fatalf("confirmation retry: %v", err)
	}
	if same, _, err := service.ChangeUsername(ctx, request, confirmed.Username); err != nil || same.UsernameChangesRemaining != 1 {
		t.Fatalf("identical username: %+v / %v", same, err)
	}
	type result struct {
		user   auth.User
		tokens []string
		err    error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for _, name := range []string{"oauth_final_one", "oauth_final_two"} {
		wait.Go(func() {
			updated, cookies, err := service.ChangeUsername(ctx, request, name)
			results <- result{updated, cookies, err}
		})
	}
	wait.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result.err == nil {
			successes++
			user = result.user
			tokens = result.tokens
			continue
		}
		var expected *auth.AuthError
		if !errors.As(result.err, &expected) || (expected.Code != "username_changes_exhausted" && expected.Code != "unauthenticated") {
			t.Fatalf("concurrent rename: %v", result.err)
		}
	}
	if successes != 1 || user.UsernameChangesRemaining != 0 {
		t.Fatalf("rename successes=%d user=%+v", successes, user)
	}
	request = profileRequest(ctx, t, tokens)
	if _, _, err := service.ChangeUsername(ctx, request, "oauth_third"); err == nil {
		t.Fatal("third username accepted")
	}
	for _, name := range []string{"显示名称一", "显示名称二"} {
		updated, err := service.ChangeDisplayName(ctx, request, name)
		if err != nil || updated.DisplayName != name || updated.UsernameChangesRemaining != 0 {
			t.Fatalf("display update: %+v / %v", updated, err)
		}
	}
	return tokens
}
