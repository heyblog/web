package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	netmail "net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/mail"
)

type Config struct {
	AccessSecret, RefreshSecret                              string
	AccessTTL, RefreshTTL, VerificationTTL, PasswordResetTTL time.Duration
	WebBaseURL, CookieDomain                                 string
	GithubClientID, GithubClientSecret, GithubScope          string
	MailFrom                                                 string
}

type Dependencies struct {
	Pool               *pgxpool.Pool
	Redis              *redis.Client
	MailSender         mail.Sender
	VerificationMailer *mail.VerificationMailer
	GithubHTTPClient   *http.Client
	Config             Config
}

type Service struct {
	repo       *repository
	redis      *redis.Client
	mailer     *mail.VerificationMailer
	sender     mail.Sender
	config     Config
	httpClient *http.Client
}

type sessionRecord struct {
	UserID      string `json:"user_id"`
	AuthVersion int32  `json:"auth_version"`
	ExpiresAt   int64  `json:"expires_at"`
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

func NewService(deps Dependencies) *Service {
	httpClient := deps.GithubHTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{repo: newRepository(deps.Pool), redis: deps.Redis, mailer: deps.VerificationMailer,
		sender: deps.MailSender, config: deps.Config, httpClient: httpClient}
}

func (service *Service) Register(ctx context.Context, username, email, password string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	email = normalizeEmail(email)
	if !validEmail(email) {
		return newAuthError("invalid_email", 422, "email is invalid")
	}
	if !usernamePattern.MatchString(username) {
		return newAuthError("invalid_username", 422, "username is invalid")
	}
	if !validPassword(password) {
		return newAuthError("invalid_password", 422, "password is invalid")
	}
	if _, err := service.repo.userByEmail(ctx, email); err == nil {
		return newAuthError("email_taken", 409, "email is already in use")
	} else if !isNotFound(err) {
		return repositoryError("find user by email", err)
	}
	if _, err := service.repo.userByUsername(ctx, username); err == nil {
		return newAuthError("username_taken", 409, "username is already in use")
	} else if !isNotFound(err) {
		return repositoryError("find user by username", err)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return repositoryError("hash password", err)
	}
	user, err := service.repo.createUser(ctx, username, email, username, &hash)
	if err != nil {
		return repositoryError("create user", err)
	}
	return service.sendVerificationCode(ctx, user)
}

func (service *Service) VerifyEmail(ctx context.Context, email, code string) error {
	if len(code) != 6 {
		return newAuthError("invalid_verification_code", 400, "verification code is invalid")
	}
	if err := service.repo.verifyEmailWithCode(ctx, normalizeEmail(email), service.codeHash(code), 5); err != nil {
		return repositoryError("verify email", err)
	}
	return nil
}

func (service *Service) ResendVerification(ctx context.Context, email string) error {
	user, err := service.repo.userByEmail(ctx, normalizeEmail(email))
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return repositoryError("find verification user", err)
	}
	if user.VerifiedAt != nil || user.PasswordHash == nil || !isActive(user) {
		return nil
	}
	return service.sendVerificationCode(ctx, user)
}

func (service *Service) sendVerificationCode(ctx context.Context, user dbUser) error {
	code, err := randomDigits(6)
	if err != nil {
		return repositoryError("generate verification code", err)
	}
	if err := service.repo.createVerificationCode(ctx, user.ID, *user.Email, service.codeHash(code), time.Now().Add(service.config.VerificationTTL)); err != nil {
		return repositoryError("store verification code", err)
	}
	if service.mailer == nil {
		return newAuthError("mail_unavailable", 503, "email delivery is unavailable")
	}
	if err := service.mailer.SendVerificationCode(ctx, *user.Email, code); err != nil {
		return repositoryError("send verification code", err)
	}
	return nil
}

func (service *Service) Login(ctx context.Context, identifier, password string) (User, []string, error) {
	identifier = strings.TrimSpace(identifier)
	var user dbUser
	var err error
	if strings.Contains(identifier, "@") {
		user, err = service.repo.userByEmail(ctx, normalizeEmail(identifier))
	} else {
		user, err = service.repo.userByUsername(ctx, strings.ToLower(identifier))
	}
	if err != nil || user.PasswordHash == nil || !isActive(user) || !verifyPassword(password, *user.PasswordHash) {
		return User{}, nil, newAuthError("invalid_credentials", 401, "invalid credentials")
	}
	if user.VerifiedAt == nil {
		return User{}, nil, newAuthError("email_not_verified", 403, "email verification is required")
	}
	if err := service.repo.recordLogin(ctx, user.ID); err != nil {
		return User{}, nil, repositoryError("record login", err)
	}
	access, refresh, err := service.issueSession(ctx, user)
	if err != nil {
		return User{}, nil, err
	}
	authUser, err := service.toUser(ctx, user)
	return authUser, []string{access, refresh}, err
}

func (service *Service) codeHash(code string) string {
	digest := sha256.Sum256([]byte(service.config.AccessSecret + ":" + code))
	return hex.EncodeToString(digest[:])
}

func normalizeEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func isActive(user dbUser) bool          { return user.AccessStatus == "ACTIVE" && user.Email != nil }
func validEmail(value string) bool {
	if len(value) > 320 {
		return false
	}
	address, err := netmail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value || !strings.Contains(value, "@") {
		return false
	}
	for index := range len(value) {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}
