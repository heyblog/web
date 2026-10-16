package databasebackup

import (
	"context"
	"net/http"
	"time"

	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type createUploadInput struct {
	Body struct {
		Size       int64    `json:"size" minimum:"1" maximum:"536870912"`
		PartSHA256 []string `json:"part_sha256" minItems:"1" maxItems:"128"`
	}
}
type uploadInput struct {
	ID string `path:"id" format:"uuid"`
}
type uploadPartInput struct {
	ID     string `path:"id" format:"uuid"`
	Number int    `path:"number" minimum:"1" maximum:"128"`
}
type currentUpload struct {
	Upload   *UploadSession `json:"upload"`
	Settings UploadSettings `json:"settings"`
}
type signedUploadPart struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func registerUploadRoutes(api huma.API, uploads *Uploads, authenticator Authenticator, token string) {
	registerDirectPartRoute(api, uploads, authenticator, token)
	operation := func(method, path, id string) huma.Operation {
		return backupOperation(method, "/uploads"+path, id, authenticator, token)
	}
	create := operation(http.MethodPost, "", "create-upload")
	create.DefaultStatus = http.StatusCreated
	httpapi.Register(api, create, func(ctx context.Context, input *createUploadInput) (*output[UploadSession], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		session, err := uploads.Create(ctx, actor, input.Body.Size, input.Body.PartSHA256)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[UploadSession]{CacheControl: "private, no-store", Body: session}, nil
	})
	httpapi.Register(api, operation(http.MethodGet, "/current", "current-upload"), func(ctx context.Context, _ *emptyInput) (*output[currentUpload], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		session, err := uploads.Current(ctx, actor)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[currentUpload]{CacheControl: "private, no-store", Body: currentUpload{session, uploads.Settings()}}, nil
	})
	httpapi.Register(api, operation(http.MethodGet, "/{id}", "get-upload"), func(ctx context.Context, input *uploadInput) (*output[UploadSession], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		session, err := uploads.Get(ctx, actor, input.ID)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[UploadSession]{CacheControl: "private, no-store", Body: session}, nil
	})
	httpapi.Register(api, operation(http.MethodPost, "/{id}/parts/{number}", "sign-upload-part"), func(ctx context.Context, input *uploadPartInput) (*output[signedUploadPart], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		signed, expires, err := uploads.Sign(ctx, actor, input.ID, input.Number)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[signedUploadPart]{CacheControl: "private, no-store", Body: signedUploadPart{signed, expires}}, nil
	})
	complete := operation(http.MethodPost, "/{id}/complete", "complete-upload")
	complete.DefaultStatus = http.StatusAccepted
	httpapi.Register(api, complete, func(ctx context.Context, input *uploadInput) (*output[UploadSession], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		session, err := uploads.Complete(ctx, actor, input.ID)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[UploadSession]{CacheControl: "private, no-store", Body: session}, nil
	})
	remove := operation(http.MethodDelete, "/{id}", "delete-upload")
	remove.DefaultStatus = http.StatusAccepted
	httpapi.Register(api, remove, func(ctx context.Context, input *uploadInput) (*output[UploadSession], error) {
		actor, _ := ctx.Value(actorKey{}).(string)
		session, err := uploads.Delete(ctx, actor, input.ID)
		if err != nil {
			return nil, httpError(err)
		}
		return &output[UploadSession]{CacheControl: "private, no-store", Body: session}, nil
	})
}
