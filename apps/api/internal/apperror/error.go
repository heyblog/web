package apperror

import "slices"

type Kind string

const (
	KindBadRequest       Kind = "bad_request"
	KindValidation       Kind = "validation"
	KindUnauthorized     Kind = "unauthorized"
	KindForbidden        Kind = "forbidden"
	KindNotFound         Kind = "not_found"
	KindMethodNotAllowed Kind = "method_not_allowed"
	KindConflict         Kind = "conflict"
	KindTooLarge         Kind = "too_large"
	KindRateLimited      Kind = "rate_limited"
	KindUnavailable      Kind = "unavailable"
	KindInternal         Kind = "internal"
)

const (
	CodeBadRequest         = "bad_request"
	CodeValidationFailed   = "validation_failed"
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeMethodNotAllowed   = "method_not_allowed"
	CodeConflict           = "conflict"
	CodeRequestTooLarge    = "request_too_large"
	CodeRateLimited        = "rate_limited"
	CodeServiceUnavailable = "service_unavailable"
	CodeInternal           = "internal_error"
)

type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Error struct {
	kind          Kind
	code          string
	publicDetail  string
	invalidParams []InvalidParam
	cause         error
	operation     string
}

func New(kind Kind, code, publicDetail string) *Error {
	return &Error{kind: kind, code: code, publicDetail: publicDetail}
}

func Wrap(cause error, kind Kind, code, publicDetail, operation string) *Error {
	return &Error{
		kind:         kind,
		code:         code,
		publicDetail: publicDetail,
		cause:        cause,
		operation:    operation,
	}
}

func (err *Error) WithInvalidParams(parameters []InvalidParam) *Error {
	copyOfError := *err
	copyOfError.invalidParams = slices.Clone(parameters)
	return &copyOfError
}

func (err *Error) Error() string {
	switch {
	case err.operation != "" && err.cause != nil:
		return err.operation + ": " + err.cause.Error()
	case err.operation != "":
		return err.operation + ": " + err.code
	case err.cause != nil:
		return err.code + ": " + err.cause.Error()
	default:
		return err.code
	}
}

func (err *Error) Unwrap() error {
	return err.cause
}

func (err *Error) Kind() Kind {
	return err.kind
}

func (err *Error) Code() string {
	return err.code
}

func (err *Error) PublicDetail() string {
	return err.publicDetail
}

func (err *Error) InvalidParams() []InvalidParam {
	return slices.Clone(err.invalidParams)
}

func (err *Error) Operation() string {
	return err.operation
}
