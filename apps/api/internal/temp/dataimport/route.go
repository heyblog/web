package dataimport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

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

type importForm struct {
	Blogs    huma.FormFile `form:"blogs" contentType:"application/json, application/octet-stream" required:"false"`
	Graph    huma.FormFile `form:"graph" contentType:"application/json, application/octet-stream" required:"false"`
	Taxonomy huma.FormFile `form:"taxonomy" contentType:"application/json, application/octet-stream" required:"false"`
}

type importInput struct {
	RawBody huma.MultipartFormFiles[importForm]
}

type importOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         importResponse
}

type importResponse struct {
	Status string       `json:"status" enum:"imported"`
	Hashes importHashes `json:"hashes"`
	Counts Counts       `json:"counts"`
}

type importHashes struct {
	Blogs    string `json:"blogs,omitempty"`
	Graph    string `json:"graph,omitempty"`
	Taxonomy string `json:"taxonomy,omitempty"`
}

func BodyLimitOverrides() map[httpapi.Route]int64 {
	return map[httpapi.Route]int64{{Method: http.MethodPost, Path: Path}: TotalBodyLimit}
}

func RegisterRoutes(api huma.API, operation ImportOperation, token string, logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	httpapi.Register(api, huma.Operation{
		OperationID:     "import-temporary-data",
		Method:          http.MethodPost,
		Path:            Path,
		Summary:         "Import temporary migration data",
		Tags:            []string{"internal migration"},
		MaxBodyBytes:    TotalBodyLimit,
		BodyReadTimeout: ImportTimeout,
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusConflict,
			http.StatusRequestEntityTooLarge,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
		Security: []map[string][]string{{"importBearer": {}}},
		Middlewares: huma.Middlewares{
			httpapi.HumaBearerAuthorization(token, "heyblog-temp-import"),
		},
	}, func(ctx context.Context, input *importInput) (*importOutput, error) {
		started := time.Now()
		deadline := started.Add(ImportTimeout)
		if err := httpapi.SetDeadlines(ctx, deadline); err != nil {
			return nil, unavailableError(err, "set import request deadlines")
		}
		if input.RawBody.Form != nil {
			defer cleanupImportForm(input.RawBody.Data(), input.RawBody.Form.RemoveAll, logger)
		}
		upload, err := decodeFormUpload(input.RawBody.Data())
		if err != nil {
			return nil, mapImportError(err)
		}
		operationContext, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		counts, err := operation.Import(operationContext, upload.Bundles)
		if err != nil {
			return nil, mapImportError(err)
		}
		logger.InfoContext(operationContext, "temporary data import completed",
			"event", "temp_data_import_completed",
			"blogs_sha256", upload.BlogsSHA256,
			"graph_sha256", upload.GraphSHA256,
			"taxonomy_sha256", upload.TaxonomySHA256,
			"sites", counts.Sites,
			"friend_links", counts.FriendLinks,
			"duration_ms", time.Since(started).Milliseconds(),
		)
		return &importOutput{
			CacheControl: "no-store",
			Body: importResponse{
				Status: "imported",
				Hashes: importHashes{
					Blogs: upload.BlogsSHA256, Graph: upload.GraphSHA256, Taxonomy: upload.TaxonomySHA256,
				},
				Counts: counts,
			},
		}, nil
	})
}

func cleanupImportForm(form *importForm, removeAll func() error, logger *slog.Logger) {
	var cleanupErr error
	if form != nil {
		for _, file := range []huma.FormFile{form.Blogs, form.Graph, form.Taxonomy} {
			if file.IsSet {
				cleanupErr = errors.Join(cleanupErr, file.Close())
			}
		}
	}
	cleanupErr = errors.Join(cleanupErr, removeAll())
	if cleanupErr != nil {
		logger.Warn("temporary data import upload cleanup failed", slog.Any("error", cleanupErr))
	}
}

func decodeFormUpload(form *importForm) (uploadedBundles, error) {
	if form == nil {
		return uploadedBundles{}, errMalformedUpload
	}
	files := make(map[string][]byte, 3)
	for _, file := range []struct {
		name  string
		value huma.FormFile
		limit int64
	}{
		{name: "blogs", value: form.Blogs, limit: BlogsFileLimit},
		{name: "graph", value: form.Graph, limit: GraphFileLimit},
		{name: "taxonomy", value: form.Taxonomy, limit: TaxonomyFileLimit},
	} {
		if !file.value.IsSet {
			continue
		}
		if file.value.Filename == "" || file.value.Size > file.limit {
			return uploadedBundles{}, errUploadTooLarge
		}
		contents, err := io.ReadAll(io.LimitReader(file.value, file.limit+1))
		if err != nil {
			return uploadedBundles{}, errMalformedUpload
		}
		if int64(len(contents)) > file.limit {
			return uploadedBundles{}, errUploadTooLarge
		}
		files[file.name] = contents
	}
	return decodeUploadedFiles(files)
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
