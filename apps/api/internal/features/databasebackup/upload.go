package databasebackup

import (
	"errors"
	"os"
)

func errorsWithCleanup(err error, file *os.File) error {
	return errors.Join(err, file.Close(), os.Remove(file.Name()))
}

type transferError struct {
	cause error
	stage string
	bytes int64
}

func (err *transferError) Error() string { return "database backup " + err.stage + " failed" }
func (err *transferError) Unwrap() error { return err.cause }
