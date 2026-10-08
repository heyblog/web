package siteaudit

import (
	"context"
	"heyblog-api/internal/features/auth"
	"net/http"
)

// AccountService owns authenticated directory workflows and delegates audit semantics.
type AccountService struct {
	repository *Repository
	audits     *Service
}

func NewAccountService(repository *Repository, audits *Service) *AccountService {
	return &AccountService{repository: repository, audits: audits}
}
func (service *AccountService) Sites(ctx context.Context, user auth.User) ([]Snapshot, error) {
	return service.repository.accountSites(ctx, user.ID)
}
func (service *AccountService) Site(ctx context.Context, user auth.User, shortID string) (Snapshot, error) {
	return service.repository.accountSite(ctx, user.ID, shortID)
}
func (service *AccountService) Friends(ctx context.Context, user auth.User, shortID string) (AccountFriends, error) {
	return service.repository.accountFriends(ctx, user.ID, shortID)
}
func (service *AccountService) SetFriend(ctx context.Context, user auth.User, shortID, targetShortID string, active bool) error {
	if shortID == targetShortID {
		return newServiceError("friend_link_self", http.StatusUnprocessableEntity, "a site cannot link to itself")
	}
	return service.repository.setAccountFriend(ctx, user.ID, shortID, targetShortID, active)
}
func (service *AccountService) CancelFriend(ctx context.Context, user auth.User, shortID, requestID string) error {
	return service.repository.cancelAccountFriend(ctx, user.ID, shortID, requestID)
}
func (service *AccountService) RemoveFriend(ctx context.Context, user auth.User, shortID, targetHost string) error {
	return service.repository.removeAccountFriend(ctx, user.ID, shortID, targetHost)
}
func (service *AccountService) Submit(ctx context.Context, user auth.User, shortID, channel string, input SubmissionInput) (SubmissionResult, error) {
	provenance := submissionProvenance{UserID: user.ID, Channel: channel}
	action := ActionCreate
	if channel != "ACCOUNT_SUBMISSION" {
		owner, err := service.repository.ownerProvenance(ctx, user.ID, shortID)
		if err != nil {
			return SubmissionResult{}, err
		}
		provenance = owner
		provenance.Channel = channel
		if channel == "OWNER_UPDATE" {
			action = ActionUpdate
		}
	}
	// Contact identity is authoritative; the caller may choose email notification only.
	input.Contact.Name = user.DisplayName
	if user.Email != nil {
		input.Contact.Email = *user.Email
	} else {
		input.Contact = ContactInput{}
	}
	targetID := ""
	if action == ActionUpdate {
		targetID = shortID
	}
	result, err := service.audits.submit(ctx, action, targetID, input, provenance)
	if err != nil {
		return SubmissionResult{}, err
	}
	if channel == "OWNER_FRIEND_LINK" {
		result.RequestID, err = service.repository.friendRequestID(ctx, result.AuditID, provenance)
		if err != nil {
			return SubmissionResult{}, err
		}
	}
	return result, nil
}
