package databasebackup

import (
	"errors"
	"io"
	"net/http"
	"os"
)

func errorsWithCleanup(err error, file *os.File) error {
	return errors.Join(err, file.Close(), os.Remove(file.Name()))
}

func upload(request *http.Request) (_ *os.File, fields map[string]string, resultErr error) {
	reader, err := request.MultipartReader()
	if err != nil {
		return nil, nil, ErrInvalid
	}
	var file *os.File
	defer func() {
		if resultErr != nil && file != nil {
			resultErr = errorsWithCleanup(resultErr, file)
		}
	}()
	fields = map[string]string{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, ErrInvalid
		}
		name := part.FormName()
		if name == "file" {
			if file != nil || part.FileName() == "" {
				return nil, nil, ErrInvalid
			}
			file, err = os.CreateTemp("", "heyblog-database-upload-*.json")
			if err != nil {
				return nil, nil, err
			}
			size, err := io.Copy(file, io.LimitReader(part, FileLimit+1))
			if err != nil {
				return nil, nil, ErrInvalid
			}
			if size > FileLimit {
				return nil, nil, ErrTooLarge
			}
		} else {
			if name != "sha256" && name != "confirmation" {
				return nil, nil, ErrInvalid
			}
			if _, exists := fields[name]; exists {
				return nil, nil, ErrInvalid
			}
			value, err := io.ReadAll(io.LimitReader(part, 129))
			if err != nil || len(value) > 128 {
				return nil, nil, ErrInvalid
			}
			fields[name] = string(value)
		}
		if err := part.Close(); err != nil {
			return nil, nil, ErrInvalid
		}
	}
	if file == nil {
		return nil, nil, ErrInvalid
	}
	return file, fields, nil
}
