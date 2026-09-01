package auth

import "net/http"

type Role string

const (
	RoleUser     Role = "USER"
	RoleAdmin    Role = "ADMIN"
	RoleSysAdmin Role = "SYS_ADMIN"
)

type Permission string

const (
	PermissionUserManage         Permission = "user.manage"
	PermissionSiteAuditReview    Permission = "site_audit.review"
	PermissionFeedbackReview     Permission = "feedback.review"
	PermissionAnnouncementManage Permission = "announcement.manage"
	PermissionTaxonomyManage     Permission = "taxonomy.manage"
	PermissionSiteManage         Permission = "site.manage"
	PermissionTaskManage         Permission = "task.manage"
	PermissionLogRead            Permission = "log.read"
)

type User struct {
	ID            string       `json:"id"`
	Email         *string      `json:"email"`
	Username      string       `json:"username"`
	DisplayName   string       `json:"display_name"`
	AvatarURL     *string      `json:"avatar_url"`
	Role          Role         `json:"role"`
	Permissions   []Permission `json:"permissions"`
	Active        bool         `json:"active"`
	EmailVerified bool         `json:"email_verified"`
	HasPassword   bool         `json:"has_password"`
	HasGitHub     bool         `json:"has_github"`
	AuthVersion   int32        `json:"auth_version"`
	CreatedAt     string       `json:"created_at"`
	LastLoginAt   *string      `json:"last_login_at"`
}

type AuthError struct {
	Code       string
	StatusCode int
	Message    string
}

func (err *AuthError) Error() string { return err.Message }

func newAuthError(code string, status int, message string) *AuthError {
	return &AuthError{Code: code, StatusCode: status, Message: message}
}

func statusKind(status int) string {
	switch status {
	case http.StatusUnprocessableEntity:
		return "validation"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "unavailable"
	default:
		return "bad_request"
	}
}
