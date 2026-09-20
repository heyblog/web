package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"heyblog-api/internal/apikey"
	"heyblog-api/internal/apperror"
	"heyblog-api/internal/httpapi"
)

type apiClientCreateRequest struct {
	Name         string          `json:"name" minLength:"1" maxLength:"128"`
	Description  string          `json:"description" maxLength:"512"`
	Audience     apikey.Audience `json:"audience" enum:"INTERNAL,EXTERNAL"`
	Scopes       []apikey.Scope  `json:"scopes" minItems:"1"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	NeverExpires bool            `json:"never_expires"`
}

type apiClientUpdateRequest struct {
	Name        string         `json:"name" minLength:"1" maxLength:"128"`
	Description string         `json:"description" maxLength:"512"`
	Scopes      []apikey.Scope `json:"scopes" minItems:"1"`
	Enabled     bool           `json:"enabled"`
}

type apiKeyRotateRequest struct {
	OverlapHours int `json:"overlap_hours" minimum:"0" maximum:"24"`
}

type apiKeyIssueRequest struct {
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	NeverExpires bool       `json:"never_expires"`
}

type apiClientPathInput[T any] struct {
	ID   string `path:"id"`
	Body T
}

type apiKeyPathInput struct {
	ID string `path:"id"`
}

type apiClientsOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Clients []apikey.Client `json:"clients"`
	}
}

type apiClientOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Client apikey.Client `json:"client"`
	}
}

type apiCredentialOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Credential apikey.Credential `json:"credential"`
	}
}

func RegisterAPIKeyManagementRoutes(
	api huma.API,
	authService *Service,
	keyService *apikey.Service,
	webToken string,
	logger *slog.Logger,
) error {
	if authService == nil || keyService == nil {
		return errors.New("authentication and API key services are required")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	operation := apiKeyManagementOperation(webToken)
	issueOperation := operation("issue-api-client-key", http.MethodPost, "/management/api-clients/{id}/keys", "Issue an API client key")
	issueOperation.DefaultStatus = http.StatusCreated
	httpapi.Register(api, issueOperation, func(ctx context.Context, input *apiClientPathInput[apiKeyIssueRequest]) (*apiCredentialOutput, error) {
		actor, err := sysAdmin(ctx, authService)
		if err != nil {
			return nil, err
		}
		credential, err := keyService.IssueKey(ctx, apikey.IssueKeyRequest{
			ClientID: input.ID, ExpiresAt: input.Body.ExpiresAt, NeverExpires: input.Body.NeverExpires, ActorID: actor.ID,
		})
		if err != nil {
			return nil, apikey.HTTPError(err)
		}
		logger.InfoContext(ctx, "API key issued", "event", "api_key_issued", "client_id", credential.Client.ID, "key_id", credential.Key.ID, "actor_id", actor.ID)
		output := &apiCredentialOutput{CacheControl: "private, no-store"}
		output.Body.Credential = credential
		return output, nil
	})

	httpapi.Register(api, operation("list-api-clients", http.MethodGet, "/management/api-clients", "List API clients"), func(ctx context.Context, _ *emptyInput) (*apiClientsOutput, error) {
		if _, err := sysAdmin(ctx, authService); err != nil {
			return nil, err
		}
		clients, err := keyService.ListClients(ctx)
		if err != nil {
			return nil, apikey.HTTPError(err)
		}
		output := &apiClientsOutput{CacheControl: "private, no-store"}
		output.Body.Clients = clients
		return output, nil
	})

	httpapi.Register(api, operation("create-api-client", http.MethodPost, "/management/api-clients", "Create an API client"), func(ctx context.Context, input *bodyInput[apiClientCreateRequest]) (*apiCredentialOutput, error) {
		actor, err := sysAdmin(ctx, authService)
		if err != nil {
			return nil, err
		}
		credential, err := keyService.CreateClient(ctx, apikey.CreateClientRequest{
			Name: input.Body.Name, Description: input.Body.Description, Audience: input.Body.Audience,
			Scopes: input.Body.Scopes, ExpiresAt: input.Body.ExpiresAt,
			NeverExpires: input.Body.NeverExpires, CreatedBy: actor.ID,
		})
		if err != nil {
			return nil, apikey.HTTPError(err)
		}
		logger.InfoContext(ctx, "API client created", "event", "api_client_created", "client_id", credential.Client.ID, "key_id", credential.Key.ID, "actor_id", actor.ID)
		output := &apiCredentialOutput{CacheControl: "private, no-store"}
		output.Body.Credential = credential
		return output, nil
	})

	httpapi.Register(api, operation("update-api-client", http.MethodPatch, "/management/api-clients/{id}", "Update an API client"), func(ctx context.Context, input *apiClientPathInput[apiClientUpdateRequest]) (*apiClientOutput, error) {
		actor, err := sysAdmin(ctx, authService)
		if err != nil {
			return nil, err
		}
		client, err := keyService.UpdateClient(ctx, apikey.UpdateClientRecord{
			ID: input.ID, Name: input.Body.Name, Description: input.Body.Description,
			Scopes: input.Body.Scopes, Enabled: input.Body.Enabled, ActorID: actor.ID,
		})
		if err != nil {
			return nil, apikey.HTTPError(err)
		}
		logger.InfoContext(ctx, "API client updated", "event", "api_client_updated", "client_id", client.ID, "actor_id", actor.ID)
		output := &apiClientOutput{CacheControl: "private, no-store"}
		output.Body.Client = client
		return output, nil
	})

	httpapi.Register(api, operation("rotate-api-client-key", http.MethodPost, "/management/api-clients/{id}/rotate", "Rotate an API client key"), func(ctx context.Context, input *apiClientPathInput[apiKeyRotateRequest]) (*apiCredentialOutput, error) {
		actor, err := sysAdmin(ctx, authService)
		if err != nil {
			return nil, err
		}
		credential, err := keyService.Rotate(ctx, apikey.RotateRequest{
			ClientID: input.ID, Overlap: time.Duration(input.Body.OverlapHours) * time.Hour, ActorID: actor.ID,
		})
		if err != nil {
			return nil, apikey.HTTPError(err)
		}
		logger.InfoContext(ctx, "API key rotated", "event", "api_key_rotated", "client_id", credential.Client.ID, "key_id", credential.Key.ID, "actor_id", actor.ID)
		output := &apiCredentialOutput{CacheControl: "private, no-store"}
		output.Body.Credential = credential
		return output, nil
	})

	httpapi.Register(api, operation("revoke-api-key", http.MethodPost, "/management/api-keys/{id}/revoke", "Revoke an API key"), func(ctx context.Context, input *apiKeyPathInput) (*noContentOutput, error) {
		actor, err := sysAdmin(ctx, authService)
		if err != nil {
			return nil, err
		}
		if err := keyService.RevokeKey(ctx, input.ID, actor.ID); err != nil {
			return nil, apikey.HTTPError(err)
		}
		logger.InfoContext(ctx, "API key revoked", "event", "api_key_revoked", "key_id", input.ID, "actor_id", actor.ID)
		return &noContentOutput{Status: http.StatusNoContent, CacheControl: "private, no-store"}, nil
	})
	return nil
}

func apiKeyManagementOperation(webToken string) func(string, string, string, string) huma.Operation {
	return func(id, method, path, summary string) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary,
			Tags: []string{"API client management"},
			Errors: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
				http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusServiceUnavailable},
			Security:     []map[string][]string{{"webToken": {}, "accessCookie": {}}},
			Middlewares:  huma.Middlewares{httpapi.HumaWebAuthorization(webToken)},
			MaxBodyBytes: 64 << 10,
		}
	}
}

func sysAdmin(ctx context.Context, service *Service) (User, error) {
	actor, err := service.Current(ctx, httpapi.Request(ctx))
	if err != nil {
		return User{}, mapError(err)
	}
	if actor.Role != RoleSysAdmin {
		return User{}, apperror.New(apperror.KindForbidden, "api_key_management_forbidden", "system administrator access is required")
	}
	return actor, nil
}
