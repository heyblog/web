package auth

import (
	"context"
	"strings"
)

type ManagedUsersFilter struct {
	Page     int32  `query:"page" default:"1" minimum:"1" maximum:"1000000"`
	PageSize int32  `query:"page_size" default:"20" minimum:"1" maximum:"100"`
	Search   string `query:"q" maxLength:"128"`
	Role     string `query:"role" enum:",USER,ADMIN,SYS_ADMIN"`
	Status   string `query:"status" enum:",ACTIVE,SUSPENDED"`
}

type ManagedUsersPage struct {
	Users    []User `json:"users"`
	Total    int64  `json:"total"`
	Page     int32  `json:"page"`
	PageSize int32  `json:"page_size"`
}

type ManagedCreate struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email" format:"email"`
}

type ManagedRevision struct {
	UpdatedAt string `json:"updated_at" minLength:"1"`
}
type ManagedProfile struct {
	ManagedRevision
	DisplayName string `json:"display_name"`
	Email       string `json:"email" format:"email"`
}
type ManagedStatus struct {
	ManagedRevision
	Status string `json:"status" enum:"ACTIVE,SUSPENDED"`
}

type ManagedMailResult struct {
	User     User `json:"user"`
	MailSent bool `json:"mail_sent"`
}

func (service *Service) ManagedUsers(ctx context.Context, actor User, filter ManagedUsersFilter) (ManagedUsersPage, error) {
	if !canManageUsers(actor) {
		return ManagedUsersPage{}, newAuthError("forbidden", 403, "user management permission is required")
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return service.repo.managedUsers(ctx, filter)
}

func (service *Service) ManagedUser(ctx context.Context, actor User, id string) (User, error) {
	if !canManageUsers(actor) {
		return User{}, newAuthError("forbidden", 403, "user management permission is required")
	}
	record, err := service.repo.userByID(ctx, id)
	if isNotFound(err) {
		return User{}, newAuthError("user_not_found", 404, "user was not found")
	}
	if err != nil {
		return User{}, err
	}
	if record.Email == nil {
		return User{}, newAuthError("user_not_found", 404, "user was not found")
	}
	return service.toUser(ctx, record)
}

func (service *Service) CreateManagedUser(ctx context.Context, actor User, input ManagedCreate) (ManagedMailResult, error) {
	if !canManageUsers(actor) {
		return ManagedMailResult{}, newAuthError("forbidden", 403, "user management permission is required")
	}
	profile, err := normalizedProfile(ProfileConfirmation{Username: input.Username, DisplayName: input.DisplayName})
	if err != nil {
		return ManagedMailResult{}, err
	}
	email := normalizeEmail(input.Email)
	if !validEmail(email) {
		return ManagedMailResult{}, newAuthError("invalid_email", 422, "email is invalid")
	}
	record, err := service.repo.createManagedUser(ctx, actor, profile, email)
	if err != nil {
		return ManagedMailResult{}, identityError(err)
	}
	sent := service.sendPasswordLink(ctx, record) == nil
	user, err := service.toUser(ctx, record)
	return ManagedMailResult{User: user, MailSent: sent}, err
}

func (service *Service) UpdateManagedProfile(ctx context.Context, actor User, id string, input ManagedProfile) (User, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	_, err := normalizedProfile(ProfileConfirmation{Username: "profile", DisplayName: input.DisplayName})
	if err != nil {
		return User{}, err
	}
	input.Email = normalizeEmail(input.Email)
	if !validEmail(input.Email) {
		return User{}, newAuthError("invalid_email", 422, "email is invalid")
	}
	record, err := service.repo.mutateManagedUser(ctx, managedMutation{Actor: actor, ID: id, UpdatedAt: input.UpdatedAt, Kind: "profile", DisplayName: input.DisplayName, Email: input.Email})
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, record)
}

func (service *Service) UpdateManagedStatus(ctx context.Context, actor User, id string, input ManagedStatus) (User, error) {
	if input.Status != "ACTIVE" && input.Status != "SUSPENDED" {
		return User{}, newAuthError("invalid_status", 422, "account status is invalid")
	}
	record, err := service.repo.mutateManagedUser(ctx, managedMutation{Actor: actor, ID: id, UpdatedAt: input.UpdatedAt, Kind: "status", Status: input.Status})
	if err != nil {
		return User{}, err
	}
	return service.toUser(ctx, record)
}

func (service *Service) ManagedAction(ctx context.Context, actor User, id string, revision ManagedRevision, kind string) (ManagedMailResult, error) {
	record, err := service.repo.mutateManagedUser(ctx, managedMutation{Actor: actor, ID: id, UpdatedAt: revision.UpdatedAt, Kind: kind})
	if err != nil {
		return ManagedMailResult{}, err
	}
	sent := true
	switch kind {
	case "password-reset":
		sent = service.sendPasswordLink(ctx, record) == nil
	case "verification":
		sent = service.sendVerificationCode(ctx, record) == nil
	}
	user, err := service.toUser(ctx, record)
	return ManagedMailResult{User: user, MailSent: sent}, err
}
