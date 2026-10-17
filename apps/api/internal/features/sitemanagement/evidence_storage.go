package sitemanagement

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	googleUUID "github.com/google/uuid"
	cos "github.com/tencentyun/cos-go-sdk-v5"

	"heyblog-api/internal/platform/config"
)

const (
	evidenceStoragePrefix = "heyblog-site-evidence/"
	evidenceUploadTTL     = 10 * time.Minute
)

var (
	ErrEvidenceStorageUnavailable = errors.New("manual evidence storage is unavailable")
	errEvidenceObjectMissing      = errors.New("manual evidence object is missing")
)

// EvidenceUpload is the short-lived direct-upload contract. The API never
// accepts file bytes only from private storage during validation.
type EvidenceUpload struct {
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// EvidenceObject is the immutable result of validating one uploaded object.
type EvidenceObject struct {
	Key         string
	Size        int64
	ContentType AttachmentKind
	SHA256      string
}

// EvidenceStorage owns the dedicated private COS prefix used for manual
// verification evidence. It intentionally shares credentials with backup COS
// while keeping object keys and retention independent.
type EvidenceStorage struct {
	client  *cos.Client
	origin  string
	prefix  string
	enabled bool
}

func NewEvidenceStorage(configuration config.BackupCOSConfig) (*EvidenceStorage, error) {
	storage := &EvidenceStorage{prefix: evidenceStoragePrefix}
	if !configuration.Enabled {
		return storage, nil
	}
	origin := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", configuration.Bucket, configuration.Region)
	endpoint, err := url.Parse(origin)
	if err != nil {
		return nil, fmt.Errorf("parse evidence COS endpoint: %w", err)
	}
	transport := &cos.AuthorizationTransport{SecretID: configuration.SecretID, SecretKey: configuration.SecretKey}
	storage.client = cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, &http.Client{Transport: transport, Timeout: 2 * time.Minute})
	storage.origin, storage.enabled = origin, true
	return storage, nil
}

func (storage *EvidenceStorage) Enabled() bool { return storage.enabled }

func (storage *EvidenceStorage) Key(id string) (string, error) {
	parsed, err := googleUUID.Parse(id)
	if err != nil || parsed.String() != id {
		return "", errors.New("invalid evidence attachment id")
	}
	return storage.prefix + id, nil
}

func (storage *EvidenceStorage) Sign(ctx context.Context, id string, input AttachmentUploadInput) (EvidenceUpload, error) {
	if !storage.enabled {
		return EvidenceUpload{}, ErrEvidenceStorageUnavailable
	}
	kind, err := input.kind()
	if err != nil {
		return EvidenceUpload{}, err
	}
	key, err := storage.Key(id)
	if err != nil {
		return EvidenceUpload{}, err
	}
	options := struct {
		Size        int64  `header:"Content-Length"`
		ContentType string `header:"Content-Type"`
		Storage     string `header:"x-cos-storage-class"`
	}{input.Size, string(kind), "STANDARD"}
	signed, err := storage.client.Object.GetPresignedURL2(ctx, http.MethodPut, key+"-upload", evidenceUploadTTL, &options, true)
	if err != nil {
		return EvidenceUpload{}, fmt.Errorf("sign evidence upload: %w", err)
	}
	return EvidenceUpload{
		URL: signed.String(),
		Headers: map[string]string{
			"Content-Length":      strconv.FormatInt(input.Size, 10),
			"Content-Type":        string(kind),
			"x-cos-storage-class": "STANDARD",
		},
		ExpiresAt: time.Now().Add(evidenceUploadTTL),
	}, nil
}

func (storage *EvidenceStorage) Complete(ctx context.Context, id string, input AttachmentUploadInput) (EvidenceObject, error) {
	if !storage.enabled {
		return EvidenceObject{}, ErrEvidenceStorageUnavailable
	}
	kind, err := input.kind()
	if err != nil {
		return EvidenceObject{}, err
	}
	key, err := storage.Key(id)
	if err != nil {
		return EvidenceObject{}, err
	}
	remote, err := storage.head(ctx, key+"-upload")
	if err != nil {
		return EvidenceObject{}, err
	}
	if remote.Size != input.Size || remote.ContentType != string(kind) {
		return EvidenceObject{}, errors.New("uploaded evidence metadata does not match declaration")
	}
	response, err := storage.client.Object.Get(ctx, key+"-upload", &cos.ObjectGetOptions{})
	if err != nil {
		return EvidenceObject{}, evidenceRemoteError(err)
	}
	defer func() { _ = response.Body.Close() }()
	content, err := io.ReadAll(io.LimitReader(response.Body, ManualPDFMaxBytes+1))
	if err != nil {
		return EvidenceObject{}, fmt.Errorf("read uploaded evidence: %w", err)
	}
	if err := ValidateAttachmentContent(input, content); err != nil {
		return EvidenceObject{}, err
	}
	digest := sha256.Sum256(content)
	checksum := hex.EncodeToString(digest[:])
	if input.SHA256 == "" || input.SHA256 != checksum {
		return EvidenceObject{}, errors.New("attachment checksum mismatch")
	}
	// Store exactly the validated bytes at a key the browser cannot write.
	// Completion is serialized by the metadata row lock before this operation.
	_, err = storage.client.Object.Put(ctx, key, bytes.NewReader(content), &cos.ObjectPutOptions{
		ACLHeaderOptions:       &cos.ACLHeaderOptions{XCosACL: "private"},
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: string(kind), ContentLength: input.Size, CacheControl: "private, no-store", ContentDisposition: "attachment"},
	})
	if err != nil {
		return EvidenceObject{}, evidenceRemoteError(err)
	}
	return EvidenceObject{Key: key, Size: int64(len(content)), ContentType: kind, SHA256: checksum}, nil
}

type evidenceRemoteObject struct {
	Size        int64
	ContentType string
}

func (storage *EvidenceStorage) head(ctx context.Context, key string) (evidenceRemoteObject, error) {
	response, err := storage.client.Object.Head(ctx, key, nil)
	if err != nil {
		return evidenceRemoteObject{}, evidenceRemoteError(err)
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return evidenceRemoteObject{}, errors.New("uploaded evidence has invalid size")
	}
	return evidenceRemoteObject{Size: size, ContentType: response.Header.Get("Content-Type")}, nil
}

func evidenceRemoteError(err error) error {
	var remote *cos.ErrorResponse
	if errors.As(err, &remote) && remote.Response != nil && remote.Response.StatusCode == http.StatusNotFound {
		return errEvidenceObjectMissing
	}
	return err
}
