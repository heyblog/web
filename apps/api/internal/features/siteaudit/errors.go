package siteaudit

import (
	"net/http"
)

type ServiceError struct {
	Code       string
	StatusCode int
	Detail     string
}

func (err *ServiceError) Error() string { return err.Code }

func newServiceError(code string, status int, detail string) *ServiceError {
	return &ServiceError{Code: code, StatusCode: status, Detail: detail}
}

func siteAddressConflictError() *ServiceError {
	return newServiceError("site_address_conflict", http.StatusConflict, "the site address is already registered")
}
