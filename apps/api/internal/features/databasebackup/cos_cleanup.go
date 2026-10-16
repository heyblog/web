package databasebackup

import (
	"context"
	"errors"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// Cleanup lists the exact generated key, including unknown initiate outcomes,
// rather than trusting a single upload ID or creating new delete markers.
func (storage *cosStorage) Cleanup(ctx context.Context, key string) error {
	ids, err := storage.Uploads(ctx, key)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := storage.client.Object.AbortMultipartUpload(ctx, key, id); err != nil && !errors.Is(remoteError(err), errRemoteMissing) {
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
	// An unversioned object may not appear in ListVersions on all endpoints.
	if _, err := storage.Head(ctx, key); err == nil {
		if _, err := storage.client.Object.Delete(ctx, key); err != nil {
			return err
		}
	} else if !errors.Is(err, errRemoteMissing) {
		return err
	}
	if _, err := storage.Head(ctx, key); !errors.Is(err, errRemoteMissing) {
		if err != nil {
			return err
		}
		return ErrBusy
	}
	return nil
}
