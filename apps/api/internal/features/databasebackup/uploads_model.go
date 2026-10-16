package databasebackup

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const UploadPartSize int64 = 4 << 20
const uploadIdleTTL = time.Hour
const uploadHardTTL = 6 * time.Hour
const uploadTombstoneTTL = 7 * 24 * time.Hour

var (
	ErrUploadUnavailable    = errors.New("backup uploads are not configured")
	ErrUploadNotFound       = errors.New("backup upload not found")
	ErrUploadState          = errors.New("backup upload is not ready for this operation")
	ErrUploadStorageChanged = errors.New("backup upload storage changed while resources remain")
)

type UploadSession struct {
	ID            string      `json:"id" format:"uuid"`
	Size          int64       `json:"size"`
	PartSize      int64       `json:"part_size"`
	PartSHA256    []string    `json:"part_sha256"`
	UploadedParts []int       `json:"uploaded_parts"`
	UploadedBytes int64       `json:"uploaded_bytes"`
	PreparedBytes int64       `json:"prepared_bytes"`
	State         string      `json:"state" enum:"uploading,preparing,preparation_failed,ready,inspecting,inspected,restoring,succeeded,failed,cancelled,expired,unknown"`
	ExpiresAt     time.Time   `json:"expires_at"`
	SHA256        string      `json:"sha256,omitempty"`
	Inspection    *Inspection `json:"inspection,omitempty"`
	JobID         string      `json:"job_id,omitempty"`
	ErrorCode     string      `json:"error_code,omitempty"`
	CleanupStatus string      `json:"cleanup_status" enum:"not_required,pending,completed"`
}

type UploadSettings struct {
	Enabled      bool   `json:"enabled"`
	Transport    string `json:"transport" enum:"cos,direct,disabled"`
	UploadOrigin string `json:"upload_origin"`
	PartSize     int64  `json:"part_size"`
	MaxFileSize  int64  `json:"max_file_size"`
}

type uploadRecord struct {
	UploadSession
	Owner                string       `json:"owner"`
	StorageIdentity      string       `json:"storage_identity"`
	Key                  string       `json:"key"`
	RemoteUploadID       string       `json:"remote_upload_id"`
	Object               remoteObject `json:"object"`
	CreatedAt            time.Time    `json:"created_at"`
	CompletedAt          time.Time    `json:"completed_at"`
	Job                  *RestoreJob  `json:"job,omitempty"`
	RemoteUncertainUntil time.Time    `json:"remote_uncertain_until"`
}

type uploadMetadata interface {
	Load(context.Context) ([]uploadRecord, error)
	Save(context.Context, uploadRecord) error
	Delete(context.Context, string) error
}

type redisUploadMetadata struct{ client *redis.Client }

const uploadRedisKey = "database-backup:uploads:v1"

func (store redisUploadMetadata) Load(ctx context.Context) ([]uploadRecord, error) {
	values, err := store.client.HGetAll(ctx, uploadRedisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("load backup uploads: %w", err)
	}
	records := make([]uploadRecord, 0, len(values))
	for _, value := range values {
		var record uploadRecord
		if err := json.Unmarshal([]byte(value), &record); err != nil {
			return nil, fmt.Errorf("decode backup upload metadata: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}

func (store redisUploadMetadata) Save(ctx context.Context, record uploadRecord) error {
	value, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode backup upload metadata: %w", err)
	}
	return store.client.HSet(ctx, uploadRedisKey, record.ID, value).Err()
}

func (store redisUploadMetadata) Delete(ctx context.Context, id string) error {
	return store.client.HDel(ctx, uploadRedisKey, id).Err()
}

func validateUpload(size int64, hashes []string) error {
	if size > FileLimit {
		return ErrTooLarge
	}
	if size <= 0 || len(hashes) != int((size+UploadPartSize-1)/UploadPartSize) {
		return ErrInvalid
	}
	for _, digest := range hashes {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != 32 {
			return ErrInvalid
		}
	}
	return nil
}

func (record *uploadRecord) touch(now time.Time) {
	record.ExpiresAt = now.Add(uploadIdleTTL)
	if maximum := record.CreatedAt.Add(uploadHardTTL); record.ExpiresAt.After(maximum) {
		record.ExpiresAt = maximum
	}
}

func (record uploadRecord) session() UploadSession {
	result := record.UploadSession
	result.PartSHA256 = append([]string{}, result.PartSHA256...)
	result.UploadedParts = append([]int{}, result.UploadedParts...)
	return result
}

func uploadTerminal(state string) bool {
	switch state {
	case "succeeded", "failed", "cancelled", "expired", "unknown":
		return true
	default:
		return false
	}
}
