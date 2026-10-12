//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
	"time"

	"heyblog-api/internal/features/auth"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/infrastructure/mail"
)

func TestAccountManagementLifecycle(t *testing.T) {
	fixture := newCredentialFixture(t)
	ctx := t.Context()
	recorder := &authMailRecorder{}
	service := auth.NewService(auth.Dependencies{Pool: fixture.pool, Redis: fixture.redis, MailSender: recorder,
		VerificationMailer: mail.NewVerificationMailer(recorder, "verify@example.test", 10*time.Minute),
		Config:             auth.Config{AccessSecret: "credential-access-test-secret", RefreshSecret: "credential-refresh-test-secret", AccessTTL: time.Hour, RefreshTTL: time.Hour, VerificationTTL: time.Hour, PasswordResetTTL: 30 * time.Minute, WebBaseURL: "http://web.example.test", MailFrom: "verify@example.test"},
	})
	if err := auth.RegisterRoutes(fixture.router.API, service, credentialWebToken); err != nil {
		t.Fatal(err)
	}
	create := fixture.request("POST", "/api/web/management/users", `{"username":"managed_user","display_name":"Managed","email":"managed@example.test"}`)
	if create.Code != http.StatusOK {
		t.Fatalf("create %d: %s", create.Code, create.Body.String())
	}
	var created auth.ManagedMailResult
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil || !created.MailSent || created.User.Role != auth.RoleUser {
		t.Fatalf("create %+v / %v", created, err)
	}
	token := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(recorder.messages[len(recorder.messages)-1].Text)[1]
	if err := service.ResetPassword(ctx, token, "managed-password"); err != nil {
		t.Fatal(err)
	}
	user, tokens, err := service.Login(ctx, "managed_user", "managed-password")
	if err != nil || !user.EmailVerified || !user.HasPassword || user.UsernameChangesRemaining != 1 {
		t.Fatalf("setup %+v / %v", user, err)
	}
	if err := service.ResetPassword(ctx, token, "second-password"); err == nil {
		t.Fatal("setup token reused")
	}
	listed := fixture.request("GET", "/api/web/management/users?page=1&page_size=1&q=managed_user&role=USER&status=ACTIVE", "")
	var page auth.ManagedUsersPage
	if err := json.Unmarshal(listed.Body.Bytes(), &page); err != nil || len(page.Users) != 1 || page.Total != 1 || page.PageSize != 1 {
		t.Fatalf("page %s / %v", listed.Body.String(), err)
	}
	path := "/api/web/management/users/" + user.ID
	if detail := fixture.request("GET", path, ""); detail.Code != 200 {
		t.Fatal(detail.Body.String())
	}
	if err := service.ForgotPassword(ctx, "managed@example.test"); err != nil {
		t.Fatal(err)
	}
	oldReset := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(recorder.messages[len(recorder.messages)-1].Text)[1]
	profileJSON, _ := json.Marshal(map[string]string{"display_name": "Updated", "email": "updated@example.test", "updated_at": user.UpdatedAt})
	profile := fixture.request("PATCH", path+"/profile", string(profileJSON))
	if profile.Code != 200 {
		t.Fatalf("profile %d: %s", profile.Code, profile.Body.String())
	}
	var output struct {
		User auth.User `json:"user"`
	}
	if err := json.Unmarshal(profile.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.User.EmailVerified || output.User.DisplayName != "Updated" {
		t.Fatalf("profile %+v", output.User)
	}
	if _, err := service.Current(ctx, profileRequest(ctx, t, tokens)); err == nil {
		t.Fatal("email change kept prior session")
	}
	if err := service.ResetPassword(ctx, oldReset, "another-password"); err == nil {
		t.Fatal("prior-email reset token accepted")
	}
	conflict := fixture.request("PATCH", path+"/profile", string(profileJSON))
	if conflict.Code != 409 {
		t.Fatalf("stale profile %d: %s", conflict.Code, conflict.Body.String())
	}
	revision, _ := json.Marshal(auth.ManagedRevision{UpdatedAt: output.User.UpdatedAt})
	verification := fixture.request("POST", path+"/verification", string(revision))
	if verification.Code != 200 {
		t.Fatal(verification.Body.String())
	}
	code := regexp.MustCompile(`[0-9]{6}`).FindString(recorder.messages[len(recorder.messages)-1].Text)
	if err := service.VerifyEmail(ctx, "updated@example.test", code); err != nil {
		t.Fatal(err)
	}
	user, tokens, err = service.Login(ctx, "managed_user", "managed-password")
	if err != nil {
		t.Fatal(err)
	}
	statusJSON, _ := json.Marshal(map[string]string{"status": "SUSPENDED", "updated_at": user.UpdatedAt})
	suspended := fixture.request("PATCH", path+"/status", string(statusJSON))
	if suspended.Code != 200 {
		t.Fatal(suspended.Body.String())
	}
	if _, err := service.Current(ctx, profileRequest(ctx, t, tokens)); err == nil {
		t.Fatal("suspension kept session")
	}
	if err := json.Unmarshal(suspended.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	statusJSON, _ = json.Marshal(map[string]string{"status": "ACTIVE", "updated_at": output.User.UpdatedAt})
	activated := fixture.request("PATCH", path+"/status", string(statusJSON))
	if activated.Code != 200 {
		t.Fatal(activated.Body.String())
	}
	user, tokens, err = service.Login(ctx, "managed_user", "managed-password")
	if err != nil {
		t.Fatal(err)
	}
	revision, _ = json.Marshal(auth.ManagedRevision{UpdatedAt: user.UpdatedAt})
	revoked := fixture.request("POST", path+"/sessions/revoke", string(revision))
	if revoked.Code != 200 {
		t.Fatal(revoked.Body.String())
	}
	if _, err := service.Current(ctx, profileRequest(ctx, t, tokens)); err == nil {
		t.Fatal("session revocation kept session")
	}
	if err := service.ForgotPassword(ctx, "updated@example.test"); err != nil {
		t.Fatal(err)
	}
	staleEmailToken := regexp.MustCompile(`token=([^\s]+)`).FindStringSubmatch(recorder.messages[len(recorder.messages)-1].Text)[1]
	if _, err := fixture.pool.Exec(ctx, "UPDATE identity.users SET email='newer@example.test' WHERE id=$1", user.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPassword(ctx, staleEmailToken, "another-password"); err == nil {
		t.Fatal("reset token accepted after its email changed")
	}
	recorder.failure = mail.ErrDeliveryUnavailable
	failedMail := fixture.request("POST", "/api/web/management/users", `{"username":"mail_retry_user","display_name":"Retry","email":"retry@example.test"}`)
	if failedMail.Code != 200 {
		t.Fatal(failedMail.Body.String())
	}
	var retry auth.ManagedMailResult
	if err := json.Unmarshal(failedMail.Body.Bytes(), &retry); err != nil || retry.MailSent || retry.User.ID == "" {
		t.Fatalf("mail failure %+v / %v", retry, err)
	}
	recorder.failure = nil
	revision, _ = json.Marshal(auth.ManagedRevision{UpdatedAt: retry.User.UpdatedAt})
	resent := fixture.request("POST", "/api/web/management/users/"+retry.User.ID+"/password-reset", string(revision))
	if resent.Code != 200 {
		t.Fatal(resent.Body.String())
	}
	if err := json.Unmarshal(resent.Body.Bytes(), &retry); err != nil || !retry.MailSent {
		t.Fatalf("mail retry %+v / %v", retry, err)
	}
	missingRevision := fixture.request("PATCH", path+"/status", `{"status":"ACTIVE"}`)
	if missingRevision.Code != 422 {
		t.Fatalf("missing revision %d: %s", missingRevision.Code, missingRevision.Body.String())
	}

	pendingRecord, err := dbgen.New(fixture.pool).GetUserByUsername(ctx, "managed_user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbgen.New(fixture.pool).RequestUserDeletion(ctx, pendingRecord.ID); err != nil {
		t.Fatal(err)
	}
	pending := fixture.request("GET", path, "")
	if err := json.Unmarshal(pending.Body.Bytes(), &output); err != nil || !output.User.DeletionPending || output.User.AccessStatus != "SUSPENDED" {
		t.Fatalf("pending deletion DTO: %s / %v", pending.Body.String(), err)
	}
	statusJSON, _ = json.Marshal(map[string]string{"status": "ACTIVE", "updated_at": output.User.UpdatedAt})
	if blocked := fixture.request("PATCH", path+"/status", string(statusJSON)); blocked.Code != 409 {
		t.Fatalf("pending deletion activation %d: %s", blocked.Code, blocked.Body.String())
	}
	profileJSON, _ = json.Marshal(map[string]string{"display_name": "Pending edit", "email": "pending@example.test", "updated_at": output.User.UpdatedAt})
	if blocked := fixture.request("PATCH", path+"/profile", string(profileJSON)); blocked.Code != 409 {
		t.Fatalf("pending deletion profile %d: %s", blocked.Code, blocked.Body.String())
	}
	verifyOtherSystemRoleManagement(t, fixture)

}

func verifyOtherSystemRoleManagement(t *testing.T, fixture credentialFixture) {
	t.Helper()
	var target string
	if err := fixture.pool.QueryRow(t.Context(), `INSERT INTO identity.users(email,username,display_name,role) VALUES ('other-system@example.test','other_system','Other system','SYS_ADMIN') RETURNING id::text`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id     string
		status int
	}{{target, 200}, {fixture.actor, 409}} {
		path := "/api/web/management/users/" + test.id
		detail := fixture.request("GET", path, "")
		var output struct {
			User auth.User `json:"user"`
		}
		if err := json.Unmarshal(detail.Body.Bytes(), &output); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(map[string]string{"role": "ADMIN", "updated_at": output.User.UpdatedAt})
		if err != nil {
			t.Fatal(err)
		}
		response := fixture.request("PATCH", path+"/role", string(body))
		if response.Code != test.status {
			t.Fatalf("system role management %d want%d: %s", response.Code, test.status, response.Body.String())
		}
	}
}
