package databasebackup

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

func errorsWithCleanup(err error, file *os.File) error {
	return errors.Join(err, file.Close(), os.Remove(file.Name()))
}

type uploadedBackup struct {
	file   *os.File
	fields map[string]string
	bytes  int64
}

type transferError struct {
	cause error
	stage string
	bytes int64
}

func (err *transferError) Error() string { return "database backup " + err.stage + " failed" }
func (err *transferError) Unwrap() error { return err.cause }

func upload(request *http.Request) (result uploadedBackup, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = &transferError{cause: errors.Join(resultErr, request.Context().Err()), stage: "upload", bytes: result.bytes}
			if result.file != nil {
				resultErr = errorsWithCleanup(resultErr, result.file)
			}
			result = uploadedBackup{}
		}
	}()
	reader, err := request.MultipartReader()
	if err != nil {
		return result, errors.Join(ErrInvalid, err)
	}
	result.fields = map[string]string{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return result, fmt.Errorf("read backup multipart: %w", err)
		}
		name := part.FormName()
		if name == "file" {
			if result.file != nil || part.FileName() == "" {
				return result, ErrInvalid
			}
			result.file, err = os.CreateTemp("", "heyblog-database-upload-*.json")
			if err != nil {
				return result, fmt.Errorf("create backup upload: %w", err)
			}
			result.bytes, err = io.Copy(result.file, io.LimitReader(part, FileLimit+1))
			if err != nil {
				return result, fmt.Errorf("receive backup upload: %w", err)
			}
			if result.bytes > FileLimit {
				return result, ErrTooLarge
			}
		} else {
			if name != "sha256" && name != "confirmation" {
				return result, ErrInvalid
			}
			if _, exists := result.fields[name]; exists {
				return result, ErrInvalid
			}
			value, err := io.ReadAll(io.LimitReader(part, 129))
			if err != nil {
				return result, fmt.Errorf("receive backup field: %w", err)
			}
			if len(value) > 128 {
				return result, ErrInvalid
			}
			result.fields[name] = string(value)
		}
		if err := part.Close(); err != nil {
			return result, fmt.Errorf("finish backup field: %w", err)
		}
	}
	if result.file == nil {
		return result, ErrInvalid
	}
	return result, nil
}
