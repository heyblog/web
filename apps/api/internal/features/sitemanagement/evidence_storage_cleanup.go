package sitemanagement

import (
	"context"
	"errors"
	"strings"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// Cleanup removes every representation of one exact key. COS versioning and
// delete markers make a plain DeleteObject insufficient for privacy cleanup.
func (storage *EvidenceStorage) Cleanup(ctx context.Context, id string) error {
	if !storage.enabled {
		return nil
	}
	key, err := storage.Key(id)
	if err != nil {
		return err
	}
	return storage.CleanupKey(ctx, key)
}

// CleanupKey removes every representation of the persisted exact key. The
// key is read from metadata so a later prefix/configuration change cannot
// accidentally target a different object.
func (storage *EvidenceStorage) CleanupKey(ctx context.Context, key string) error {
	if !storage.enabled {
		return nil
	}
	if !storage.validKey(key) {
		return errors.New("invalid evidence storage key")
	}
	uploads, err := storage.listMultipartUploads(ctx, key)
	if err != nil {
		return err
	}
	for _, uploadID := range uploads {
		if _, err := storage.client.Object.AbortMultipartUpload(ctx, key, uploadID); err != nil && !errors.Is(evidenceRemoteError(err), errEvidenceObjectMissing) {
			return err
		}
	}
	options := &cos.BucketGetObjectVersionsOptions{Prefix: key, MaxKeys: 1000}
	for {
		result, _, err := storage.client.Bucket.GetObjectVersions(ctx, options)
		if err != nil {
			return err
		}
		for _, version := range result.Version {
			if version.Key == key {
				if _, err := storage.client.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: version.VersionId}); err != nil {
					return err
				}
			}
		}
		for _, marker := range result.DeleteMarker {
			if marker.Key == key {
				if _, err := storage.client.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: marker.VersionId}); err != nil {
					return err
				}
			}
		}
		if !result.IsTruncated {
			break
		}
		options.KeyMarker, options.VersionIdMarker = result.NextKeyMarker, result.NextVersionIdMarker
	}
	if _, err := storage.client.Object.Head(ctx, key, nil); err == nil {
		if _, err := storage.client.Object.Delete(ctx, key); err != nil {
			return err
		}
	} else if !errors.Is(evidenceRemoteError(err), errEvidenceObjectMissing) {
		return err
	}
	if _, err := storage.client.Object.Head(ctx, key, nil); !errors.Is(evidenceRemoteError(err), errEvidenceObjectMissing) {
		if err != nil {
			return err
		}
		return errors.New("evidence object remains after cleanup")
	}
	return nil
}

// Archive moves closed evidence to immediately readable infrequent-access storage.
// The object key remains stable, so reviewers and cleanup jobs keep the same
// durable locator throughout the retention lifecycle.
func (storage *EvidenceStorage) Archive(ctx context.Context, key string) error {
	if !storage.enabled {
		return ErrEvidenceStorageUnavailable
	}
	if !storage.validKey(key) {
		return errors.New("invalid evidence storage key")
	}
	_, _, err := storage.client.Object.Copy(ctx, key, storage.origin+"/"+key, &cos.ObjectCopyOptions{
		ObjectCopyHeaderOptions: &cos.ObjectCopyHeaderOptions{XCosStorageClass: "STANDARD_IA"},
	})
	if err != nil {
		return evidenceRemoteError(err)
	}
	return nil
}

func (storage *EvidenceStorage) listMultipartUploads(ctx context.Context, key string) ([]string, error) {
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

func (storage *EvidenceStorage) validKey(key string) bool {
	id := strings.TrimSuffix(strings.TrimPrefix(key, storage.prefix), "-upload")
	expected, err := storage.Key(id)
	return err == nil && (key == expected || key == expected+"-upload")
}
