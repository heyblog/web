package databasebackup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"heyblog-api/internal/platform/config"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

var errRemoteMissing = errors.New("backup remote object missing")

type remotePart struct {
	Number int
	Size   int64
	ETag   string
}
type remoteObject struct {
	ETag, Version string
	Size          int64
}
type uploadStorage interface {
	Identity() string
	Initiate(context.Context, string) (string, error)
	Uploads(context.Context, string) ([]string, error)
	Parts(context.Context, string, string) ([]remotePart, error)
	Sign(context.Context, string, string, int, int64) (string, error)
	Complete(context.Context, string, string, []remotePart) (remoteObject, error)
	Head(context.Context, string) (remoteObject, error)
	Open(context.Context, string, remoteObject) (io.ReadCloser, error)
	Cleanup(context.Context, string) error
}

type cosStorage struct{ client *cos.Client }

func (storage *cosStorage) Identity() string { return storage.client.BaseURL.BucketURL.String() }

func newCOSStorage(configuration config.BackupCOSConfig) (*cosStorage, string, error) {
	origin := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", configuration.Bucket, configuration.Region)
	endpoint, err := url.Parse(origin)
	if err != nil {
		return nil, "", err
	}
	transport := &cos.AuthorizationTransport{SecretID: configuration.SecretID, SecretKey: configuration.SecretKey}
	client := cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, &http.Client{Transport: transport, Timeout: Timeout})
	return &cosStorage{client: client}, origin, nil
}

func (storage *cosStorage) Initiate(ctx context.Context, key string) (string, error) {
	result, _, err := storage.client.Object.InitiateMultipartUpload(ctx, key, &cos.InitiateMultipartUploadOptions{
		ACLHeaderOptions:       &cos.ACLHeaderOptions{XCosACL: "private"},
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: "application/json", XCosStorageClass: "STANDARD"},
	})
	if err != nil {
		return "", err
	}
	return result.UploadID, nil
}

func (storage *cosStorage) Uploads(ctx context.Context, key string) ([]string, error) {
	options := &cos.ListMultipartUploadsOptions{Prefix: key, MaxUploads: 1000}
	var ids []string
	for {
		result, _, err := storage.client.Bucket.ListMultipartUploads(ctx, options)
		if err != nil {
			return nil, err
		}
		for _, upload := range result.Uploads {
			if upload.Key == key {
				ids = append(ids, upload.UploadID)
			}
		}
		if !result.IsTruncated {
			return ids, nil
		}
		options.KeyMarker, options.UploadIDMarker = result.NextKeyMarker, result.NextUploadIDMarker
	}
}

func (storage *cosStorage) Parts(ctx context.Context, key, uploadID string) ([]remotePart, error) {
	result, _, err := storage.client.Object.ListParts(ctx, key, uploadID, &cos.ObjectListPartsOptions{MaxParts: "1000"})
	if err != nil {
		return nil, remoteError(err)
	}
	if result.IsTruncated {
		return nil, ErrInvalid
	}
	parts := make([]remotePart, 0, len(result.Parts))
	for _, part := range result.Parts {
		parts = append(parts, remotePart{part.PartNumber, part.Size, part.ETag})
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Number < parts[j].Number })
	return parts, nil
}

func (storage *cosStorage) Sign(ctx context.Context, key, uploadID string, number int, size int64) (string, error) {
	options := struct {
		UploadID string `url:"uploadId" header:"-"`
		Number   int    `url:"partNumber" header:"-"`
		Size     int64  `url:"-" header:"Content-Length"`
	}{uploadID, number, size}
	signed, err := storage.client.Object.GetPresignedURL2(ctx, http.MethodPut, key, 10*time.Minute, &options, true)
	if err != nil {
		return "", err
	}
	return signed.String(), nil
}

func (storage *cosStorage) Complete(ctx context.Context, key, uploadID string, parts []remotePart) (remoteObject, error) {
	objects := make([]cos.Object, 0, len(parts))
	for _, part := range parts {
		objects = append(objects, cos.Object{PartNumber: part.Number, ETag: part.ETag})
	}
	_, _, err := storage.client.Object.CompleteMultipartUpload(ctx, key, uploadID, &cos.CompleteMultipartUploadOptions{Parts: objects})
	if err != nil {
		return remoteObject{}, remoteError(err)
	}
	return storage.Head(ctx, key)
}

func (storage *cosStorage) Head(ctx context.Context, key string) (remoteObject, error) {
	response, err := storage.client.Object.Head(ctx, key, nil)
	if err != nil {
		return remoteObject{}, remoteError(err)
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return remoteObject{}, ErrInvalid
	}
	return remoteObject{ETag: response.Header.Get("ETag"), Version: response.Header.Get("x-cos-version-id"), Size: size}, nil
}

func (storage *cosStorage) Open(ctx context.Context, key string, object remoteObject) (io.ReadCloser, error) {
	headers := http.Header{"If-Match": []string{object.ETag}}
	options := &cos.ObjectGetOptions{XOptionHeader: &headers}
	var response *cos.Response
	var err error
	if object.Version != "" {
		response, err = storage.client.Object.Get(ctx, key, options, object.Version)
	} else {
		response, err = storage.client.Object.Get(ctx, key, options)
	}
	if err != nil {
		return nil, remoteError(err)
	}
	return response.Body, nil
}

func remoteError(err error) error {
	var upstream *cos.ErrorResponse
	if errors.As(err, &upstream) && upstream.Response != nil && upstream.Response.StatusCode == http.StatusNotFound {
		return errRemoteMissing
	}
	return err
}
