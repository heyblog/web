package siteaudit

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/auth"
	"heyblog-api/internal/httpapi"
)

type bodyInput[T any] struct {
	Body T
}

type shortIDBodyInput[T any] struct {
	ShortID string `path:"shortId"`
	Body    T
}

type auditIDBodyInput[T any] struct {
	AuditID string `path:"auditId"`
	Body    T
}

type shortIDInput struct {
	ShortID string `path:"shortId"`
}

type auditIDInput struct {
	AuditID string `path:"auditId"`
}

type searchInput struct {
	Query string `query:"q" required:"true" minLength:"1" maxLength:"160"`
}

type availabilityInput struct {
	URL string `query:"url" required:"true"`
}

type managementListInput struct {
	Status   string `query:"status" enum:"PENDING,APPROVED,REJECTED"`
	Action   string `query:"action" enum:"CREATE,UPDATE,DELETE,RESTORE"`
	Page     string `query:"page"`
	PageSize string `query:"page_size"`
}

type lookupRequest struct {
	LookupToken string `json:"lookup_token"`
}

type bodyOutput[T any] struct {
	Body T
}

type searchResponse struct {
	Items []SiteSearchResult `json:"items"`
}

func submitHandler(service *Service, action Action) func(context.Context, *shortIDBodyInput[SubmissionInput]) (*bodyOutput[SubmissionResult], error) {
	return func(ctx context.Context, input *shortIDBodyInput[SubmissionInput]) (*bodyOutput[SubmissionResult], error) {
		result, err := service.Submit(ctx, action, input.ShortID, input.Body)
		if err != nil {
			return nil, mapServiceError(err, "submit site audit")
		}
		return &bodyOutput[SubmissionResult]{Body: result}, nil
	}
}

func createSubmitHandler(service *Service) func(context.Context, *bodyInput[SubmissionInput]) (*bodyOutput[SubmissionResult], error) {
	return func(ctx context.Context, input *bodyInput[SubmissionInput]) (*bodyOutput[SubmissionResult], error) {
		result, err := service.Submit(ctx, ActionCreate, "", input.Body)
		if err != nil {
			return nil, mapServiceError(err, "submit site audit")
		}
		return &bodyOutput[SubmissionResult]{Body: result}, nil
	}
}

func queryHandler(service *Service) func(context.Context, *bodyInput[lookupRequest]) (*bodyOutput[PublicAuditResult], error) {
	return func(ctx context.Context, input *bodyInput[lookupRequest]) (*bodyOutput[PublicAuditResult], error) {
		result, err := service.Query(ctx, input.Body.LookupToken)
		if err != nil {
			return nil, mapServiceError(err, "query site audit")
		}
		return &bodyOutput[PublicAuditResult]{Body: result}, nil
	}
}

func optionsHandler(service *Service) func(context.Context, *struct{}) (*bodyOutput[SubmissionOptions], error) {
	return func(ctx context.Context, _ *struct{}) (*bodyOutput[SubmissionOptions], error) {
		options, err := service.Options(ctx)
		if err != nil {
			return nil, mapServiceError(err, "list site submission options")
		}
		return &bodyOutput[SubmissionOptions]{Body: options}, nil
	}
}

func searchHandler(service *Service) func(context.Context, *searchInput) (*bodyOutput[searchResponse], error) {
	return func(ctx context.Context, input *searchInput) (*bodyOutput[searchResponse], error) {
		query := strings.TrimSpace(input.Query)
		if query == "" || len(query) > 160 {
			return nil, apperror.New(apperror.KindValidation, "invalid_search", "the site search query is invalid")
		}
		results, err := service.SearchSites(ctx, query)
		if err != nil {
			return nil, mapServiceError(err, "search sites for submission")
		}
		return &bodyOutput[searchResponse]{Body: searchResponse{Items: results}}, nil
	}
}

func availabilityHandler(service *Service) func(context.Context, *availabilityInput) (*bodyOutput[SiteAvailability], error) {
	return func(ctx context.Context, input *availabilityInput) (*bodyOutput[SiteAvailability], error) {
		result, err := service.CheckSiteAvailability(ctx, input.URL)
		if err != nil {
			return nil, mapServiceError(err, "check site availability")
		}
		return &bodyOutput[SiteAvailability]{Body: result}, nil
	}
}

func resolveHandler(service *Service) func(context.Context, *shortIDInput) (*bodyOutput[Snapshot], error) {
	return func(ctx context.Context, input *shortIDInput) (*bodyOutput[Snapshot], error) {
		snapshot, err := service.ResolveSite(ctx, input.ShortID)
		if err != nil {
			return nil, mapServiceError(err, "resolve site for submission")
		}
		return &bodyOutput[Snapshot]{Body: snapshot}, nil
	}
}

func managementListHandler(service *Service) func(context.Context, *managementListInput) (*bodyOutput[AuditPage], error) {
	return func(ctx context.Context, input *managementListInput) (*bodyOutput[AuditPage], error) {
		if _, err := service.CurrentReviewer(ctx, httpapi.Request(ctx)); err != nil {
			return nil, mapServiceError(err, "authorize site audit listing")
		}
		status, action, err := parseFilters(input.Status, input.Action)
		if err != nil {
			return nil, err
		}
		result, err := service.ListAudits(
			ctx,
			status,
			action,
			boundedInteger(input.Page, 1, 1, 1_000_000),
			boundedInteger(input.PageSize, 20, 1, 50),
		)
		if err != nil {
			return nil, mapServiceError(err, "list site audits")
		}
		return &bodyOutput[AuditPage]{Body: result}, nil
	}
}

func managementDetailHandler(service *Service) func(context.Context, *auditIDInput) (*bodyOutput[Audit], error) {
	return func(ctx context.Context, input *auditIDInput) (*bodyOutput[Audit], error) {
		if _, err := service.CurrentReviewer(ctx, httpapi.Request(ctx)); err != nil {
			return nil, mapServiceError(err, "authorize site audit detail")
		}
		audit, err := service.AuditDetail(ctx, input.AuditID)
		if err != nil {
			return nil, mapServiceError(err, "get site audit detail")
		}
		return &bodyOutput[Audit]{Body: audit}, nil
	}
}

func parseFilters(rawStatus, rawAction string) (*Status, *Action, error) {
	var status *Status
	if rawStatus != "" {
		value := Status(rawStatus)
		if value != StatusPending && value != StatusApproved && value != StatusRejected {
			return nil, nil, apperror.New(apperror.KindValidation, "invalid_status", "the audit status filter is invalid")
		}
		status = &value
	}
	var action *Action
	if rawAction != "" {
		value := Action(rawAction)
		if value != ActionCreate && value != ActionUpdate && value != ActionDelete && value != ActionRestore {
			return nil, nil, apperror.New(apperror.KindValidation, "invalid_action", "the audit action filter is invalid")
		}
		action = &value
	}
	return status, action, nil
}

func boundedInteger(raw string, fallback, minimum, maximum int32) int32 {
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || parsed < int64(minimum) || parsed > int64(maximum) {
		return fallback
	}
	return int32(parsed)
}

func mapServiceError(err error, operation string) error {
	var authError *auth.AuthError
	if errors.As(err, &authError) {
		kind := apperror.KindUnauthorized
		if authError.StatusCode == http.StatusForbidden {
			kind = apperror.KindForbidden
		}
		return apperror.Wrap(err, kind, authError.Code, authError.Message, operation)
	}
	var serviceError *ServiceError
	if errors.As(err, &serviceError) {
		kind := apperror.KindBadRequest
		switch serviceError.StatusCode {
		case http.StatusForbidden:
			kind = apperror.KindForbidden
		case http.StatusNotFound:
			kind = apperror.KindNotFound
		case http.StatusConflict:
			kind = apperror.KindConflict
		case http.StatusUnprocessableEntity:
			kind = apperror.KindValidation
		}
		return apperror.Wrap(err, kind, serviceError.Code, serviceError.Detail, operation)
	}
	if errors.Is(err, ErrInvalidSubmission) {
		return apperror.Wrap(err, apperror.KindValidation, "invalid_submission", "the site submission is invalid", operation)
	}
	return apperror.Wrap(err, apperror.KindInternal, apperror.CodeInternal, "the site audit operation failed", operation).
		WithDiagnostics(siteAuditDiagnostics(err))
}
