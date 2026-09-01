package publicview

import (
	"errors"

	"heyblog-api/internal/apperror"
)

func badIdentifier(name string) error {
	return apperror.New(
		apperror.KindBadRequest,
		apperror.CodeBadRequest,
		"site identifier is invalid",
	).WithInvalidParams([]apperror.InvalidParam{{
		Name: name, Reason: "must use the accepted route format",
	}})
}

func notFound() error {
	return apperror.New(apperror.KindNotFound, apperror.CodeNotFound, "site was not found")
}

func internalError(err error, operation string) error {
	return apperror.Wrap(
		err,
		apperror.KindInternal,
		apperror.CodeInternal,
		"unable to load public site data",
		operation,
	)
}

func invalidSiteTimestamp(name string) error {
	return errors.New("site " + name + " time is invalid")
}
