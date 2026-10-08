package databasebackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (service *Service) Inspect(ctx context.Context, file *os.File, actor string) (Inspection, error) {
	preview, _, err := service.inspect(ctx, file, actor)
	return preview, err
}

func (service *Service) inspect(ctx context.Context, file *os.File, actor string) (Inspection, document, error) {
	info, err := file.Stat()
	if err != nil {
		return Inspection{}, document{}, err
	}
	if info.Size() > FileLimit {
		return Inspection{}, document{}, ErrTooLarge
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return Inspection{}, document{}, err
	}
	hash := sha256.New()
	decoder := json.NewDecoder(io.TeeReader(file, hash))
	decoder.UseNumber()
	if err = scanJSON(decoder, 0); err != nil {
		return Inspection{}, document{}, err
	}
	if err = requireEOF(decoder); err != nil {
		return Inspection{}, document{}, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return Inspection{}, document{}, err
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Inspection{}, document{}, err
	}
	defer rollback(ctx, tx)
	inventory, err := columns(ctx, tx)
	if err != nil {
		return Inspection{}, document{}, err
	}
	doc, err := readDocument(file, func(name string, row json.RawMessage) error { return validateRow(name, row, inventory) })
	if err != nil {
		return Inspection{}, document{}, err
	}
	preview := doc.inspection(checksum, actor)
	preview.Issues, err = targetIssues(ctx, tx, actor)
	if err != nil {
		return Inspection{}, document{}, err
	}
	preview.TargetReady = len(preview.Issues) == 0
	preview.CanRestore = preview.TargetReady
	return preview, doc, tx.Commit(ctx)
}

func validateRow(name string, raw json.RawMessage, inventory columnInventory) error {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		return ErrInvalid
	}
	fields := inventory[name]
	if strings.HasPrefix(name, "graph.") {
		if name == "graph.vertices" {
			fields = map[string]string{"normalized_host": "text", "site_id": "uuid"}
		} else {
			fields = map[string]string{"source_host": "text", "target_host": "text", "target_url": "text", "status": "text", "created_at_ms": "bigint", "updated_at_ms": "bigint"}
		}
	}
	if len(row) != len(fields) {
		return ErrInvalid
	}
	for field, rawValue := range row {
		typeName, ok := fields[field]
		if !ok {
			return ErrInvalid
		}
		if bytes.Equal(rawValue, []byte("null")) {
			continue
		}
		switch typeName {
		case "bigint", "bytea":
			var value string
			if err := json.Unmarshal(rawValue, &value); err != nil {
				return ErrInvalid
			}
			if typeName == "bigint" {
				if _, err := strconv.ParseInt(value, 10, 64); err != nil {
					return ErrInvalid
				}
			} else {
				if _, err := base64.StdEncoding.DecodeString(value); err != nil {
					return ErrInvalid
				}
			}
		}
	}
	return nil
}
