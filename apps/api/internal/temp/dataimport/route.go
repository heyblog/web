package dataimport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/httpapi"
)

const (
	Path          = "/internal/temp/data-import"
	ImportTimeout = 90 * time.Minute
)

type ImportOperation interface {
	Import(context.Context, Bundles) (Counts, error)
}

type importResponse struct {
	Status string       `json:"status"`
	Hashes importHashes `json:"hashes"`
	Counts Counts       `json:"counts"`
}

type importHashes struct {
	Blogs string `json:"blogs"`
	Graph string `json:"graph"`
}

func BodyLimitOverrides() map[httpapi.Route]int64 {
	return map[httpapi.Route]int64{{Method: http.MethodPost, Path: Path}: TotalBodyLimit}
}

func RegisterRoutes(router *gin.Engine, operation ImportOperation, token string, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	endpoint := func(ctx *httpapi.Context) (httpapi.Response, error) {
		started := time.Now()
		deadline := started.Add(ImportTimeout)
		if err := ctx.SetReadDeadline(deadline); err != nil {
			return httpapi.Response{}, unavailableError(err, "set import read deadline")
		}
		if err := ctx.SetWriteDeadline(deadline); err != nil {
			return httpapi.Response{}, unavailableError(err, "set import write deadline")
		}
		upload, err := decodeUpload(ctx.Request)
		if err != nil {
			return httpapi.Response{}, mapImportError(err)
		}
		operationContext, cancel := context.WithDeadline(ctx.Request.Context(), deadline)
		defer cancel()
		counts, err := operation.Import(operationContext, upload.Bundles)
		if err != nil {
			return httpapi.Response{}, mapImportError(err)
		}
		logger.InfoContext(operationContext, "temporary data import completed",
			"event", "temp_data_import_completed",
			"blogs_sha256", upload.BlogsSHA256,
			"graph_sha256", upload.GraphSHA256,
			"sites", counts.Sites,
			"friend_links", counts.FriendLinks,
			"duration_ms", time.Since(started).Milliseconds(),
		)
		response, responseErr := httpapi.JSON(http.StatusOK, importResponse{
			Status: "imported",
			Hashes: importHashes{Blogs: upload.BlogsSHA256, Graph: upload.GraphSHA256},
			Counts: counts,
		})
		return response.WithHeader("Cache-Control", "no-store"), responseErr
	}
	router.POST(Path, httpapi.Adapt(httpapi.Chain(
		endpoint,
		httpapi.BearerAuthorization(token, "heyblog-temp-import"),
	)))
}

func mapImportError(err error) error {
	switch {
	case errors.Is(err, errMalformedUpload):
		return apperror.Wrap(err, apperror.KindBadRequest, apperror.CodeBadRequest, "multipart upload is invalid", "decode temporary import upload")
	case errors.Is(err, errUploadTooLarge):
		return apperror.Wrap(err, apperror.KindTooLarge, apperror.CodeRequestTooLarge, "uploaded data exceeds the allowed size", "decode temporary import upload")
	case errors.Is(err, errInvalidContract), errors.Is(err, ErrInvalidBundle):
		return apperror.Wrap(err, apperror.KindValidation, apperror.CodeValidationFailed, "cleaned import data is invalid", "validate temporary import data")
	case errors.Is(err, ErrImportRunning):
		return apperror.Wrap(err, apperror.KindConflict, apperror.CodeConflict, "a data import is already running", "start temporary import")
	case errors.Is(err, ErrDirectoryNotEmpty):
		return apperror.Wrap(err, apperror.KindConflict, apperror.CodeConflict, "the directory already contains data", "start temporary import")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrDependencyUnavailable):
		return unavailableError(err, "run temporary import")
	default:
		return apperror.Wrap(err, apperror.KindInternal, apperror.CodeInternal, "data import failed", "run temporary import")
	}
}

func unavailableError(err error, operation string) error {
	return apperror.Wrap(err, apperror.KindUnavailable, apperror.CodeServiceUnavailable, "data import dependency is unavailable", operation)
}
