package siteaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/auth"
	"heyblog-api/internal/httpapi"
)

type lookupRequest struct {
	LookupToken string `json:"lookup_token"`
}

func submitEndpoint(service *Service, action Action) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		var input SubmissionInput
		if err := decodeRequest(ctx.Request, &input); err != nil {
			return httpapi.Response{}, err
		}
		result, err := service.Submit(ctx.Request.Context(), action, ctx.Param("shortId"), input)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusCreated, result)
	}
}

func queryEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		var input lookupRequest
		if err := decodeRequest(ctx.Request, &input); err != nil {
			return httpapi.Response{}, err
		}
		result, err := service.Query(ctx.Request.Context(), input.LookupToken)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, result)
	}
}

func optionsEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		options, err := service.Options(ctx.Request.Context())
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, options)
	}
}

func searchEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		query := strings.TrimSpace(ctx.Request.URL.Query().Get("q"))
		if query == "" || len(query) > 160 {
			return httpapi.Response{}, apperror.New(apperror.KindValidation, "invalid_search", "the site search query is invalid")
		}
		results, err := service.SearchSites(ctx.Request.Context(), query)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, map[string][]SiteSearchResult{"items": results})
	}
}

func resolveEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		snapshot, err := service.ResolveSite(ctx.Request.Context(), ctx.Param("shortId"))
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, snapshot)
	}
}

func managementListEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		if _, err := service.CurrentReviewer(ctx.Request.Context(), ctx.Request); err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		status, action, err := parseFilters(ctx.Request.URL.Query().Get("status"), ctx.Request.URL.Query().Get("action"))
		if err != nil {
			return httpapi.Response{}, err
		}
		page := boundedInteger(ctx.Request.URL.Query().Get("page"), 1, 1, 1_000_000)
		pageSize := boundedInteger(ctx.Request.URL.Query().Get("page_size"), 20, 1, 50)
		result, err := service.ListAudits(ctx.Request.Context(), status, action, page, pageSize)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, result)
	}
}

func managementDetailEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		if _, err := service.CurrentReviewer(ctx.Request.Context(), ctx.Request); err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		audit, err := service.AuditDetail(ctx.Request.Context(), ctx.Param("auditId"))
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, audit)
	}
}

func managementReviewEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		reviewer, err := service.CurrentReviewer(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		var input ReviewInput
		if err := decodeRequest(ctx.Request, &input); err != nil {
			return httpapi.Response{}, err
		}
		input.AuditID = ctx.Param("auditId")
		audit, err := service.Review(ctx.Request.Context(), reviewer, input)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, audit)
	}
}

func managementSaveReviewDraftEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		reviewer, err := service.CurrentReviewer(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		var input ReviewDraftInput
		if err := decodeRequest(ctx.Request, &input); err != nil {
			return httpapi.Response{}, err
		}
		input.AuditID = ctx.Param("auditId")
		audit, err := service.SaveReviewDraft(ctx.Request.Context(), reviewer, input)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, audit)
	}
}

func managementDiscardReviewDraftEndpoint(service *Service) httpapi.Endpoint {
	return func(ctx *httpapi.Context) (httpapi.Response, error) {
		reviewer, err := service.CurrentReviewer(ctx.Request.Context(), ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		var input DiscardReviewDraftInput
		if err := decodeRequest(ctx.Request, &input); err != nil {
			return httpapi.Response{}, err
		}
		input.AuditID = ctx.Param("auditId")
		audit, err := service.DiscardReviewDraft(ctx.Request.Context(), reviewer, input)
		if err != nil {
			return httpapi.Response{}, mapServiceError(err)
		}
		return httpapi.JSON(http.StatusOK, audit)
	}
}

func decodeRequest(request *http.Request, destination any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apperror.Wrap(err, apperror.KindBadRequest, "invalid_json", "the request body is not valid JSON", "decode site audit request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.New(apperror.KindBadRequest, "invalid_json", "the request body must contain one JSON value")
	}
	return nil
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

func mapServiceError(err error) error {
	var authError *auth.AuthError
	if errors.As(err, &authError) {
		kind := apperror.KindUnauthorized
		if authError.StatusCode == http.StatusForbidden {
			kind = apperror.KindForbidden
		}
		return apperror.New(kind, authError.Code, authError.Message)
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
		return apperror.New(kind, serviceError.Code, serviceError.Detail)
	}
	if errors.Is(err, ErrInvalidSubmission) {
		return apperror.Wrap(err, apperror.KindValidation, "invalid_submission", "the site submission is invalid", "validate site submission")
	}
	return apperror.Wrap(err, apperror.KindInternal, apperror.CodeInternal, "the site audit operation failed", fmt.Sprintf("site audit %T", err))
}
