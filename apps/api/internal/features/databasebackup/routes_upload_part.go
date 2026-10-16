package databasebackup

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type uploadPartOutput struct {
	CacheControl string `header:"Cache-Control"`
}

type interruptPartBody struct {
	io.ReadCloser
	ctx context.Context
}

func (body interruptPartBody) Close() error {
	// A server request body may be blocked in a socket read. Interrupt that
	// read before Close so cancellation and shutdown can join the writer.
	err := httpapi.SetDeadlines(body.ctx, time.Now())
	if errors.Is(err, http.ErrNotSupported) {
		err = nil
	}
	return errors.Join(err, body.ReadCloser.Close())
}

func registerDirectPartRoute(api huma.API, uploads *Uploads, authenticator Authenticator, token string) {
	operation := backupOperation(http.MethodPut, "/uploads/{id}/parts/{number}", "receive-upload-part", authenticator, token)
	operation.MaxBodyBytes, operation.BodyReadTimeout, operation.DefaultStatus = UploadPartSize, 2*time.Minute, http.StatusNoContent
	operation.RequestBody = &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{
		"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}},
	}}
	httpapi.Register(api, operation, func(ctx context.Context, input *uploadPartInput) (*uploadPartOutput, error) {
		request := httpapi.Request(ctx)
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/octet-stream" {
			return nil, httpError(ErrInvalid)
		}
		if request.ContentLength > UploadPartSize {
			return nil, httpError(ErrTooLarge)
		}
		if err := httpapi.SetDeadlines(ctx, time.Now().Add(2*time.Minute)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return nil, httpError(err)
		}
		actor, _ := ctx.Value(actorKey{}).(string)
		if err := uploads.ReceivePart(ctx, UploadPart{Owner: actor, ID: input.ID, Number: input.Number, Body: interruptPartBody{ReadCloser: request.Body, ctx: ctx}}); err != nil {
			return nil, httpError(err)
		}
		return &uploadPartOutput{CacheControl: "private, no-store"}, nil
	})
}
