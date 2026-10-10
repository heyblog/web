package databasebackup

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5"
)

type jsonWriter struct {
	writer io.Writer
	err    error
	bytes  int64
}

func (writer *jsonWriter) write(value string) {
	if writer.err != nil {
		return
	}
	writer.bytes += int64(len(value))
	if writer.bytes > FileLimit {
		writer.err = ErrTooLarge
		return
	}
	_, writer.err = io.WriteString(writer.writer, value)
}

func (writer *jsonWriter) value(value any) {
	if writer.err != nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		writer.err = err
		return
	}
	writer.write(string(data))
}

// Export materializes one consistent snapshot in a private temporary file before
// sending headers, so database failures never produce a successful partial file.
// The caller owns closing and removing the returned file.
func (service *Service) Export(ctx context.Context) (_ *os.File, resultErr error) {
	file, err := os.CreateTemp("", "heyblog-database-backup-*.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errorsWithCleanup(resultErr, file)
		}
	}()
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)
	admin, err := sourceAdmin(ctx, tx)
	if err != nil {
		return nil, err
	}
	buffer := bufio.NewWriter(file)
	writer := &jsonWriter{writer: buffer}
	writer.write("{\n\"metadata\":")
	writer.value(Header{Format: Format, Version: 1, SchemaVersion: 3, GeneratedAt: time.Now().UTC(), SourceRevision: buildRevision(), ExcludedSystemAdminID: admin})
	writer.write(",\n\"tables\":{")
	manifest := make([]Manifest, 0, len(tableNames)+2)
	for index, name := range tableNames {
		if index > 0 {
			writer.write(",")
		}
		writer.write("\n")
		writer.value(name)
		writer.write(":[\n")
		rows, err := tx.Query(ctx, "SELECT directory.backup_table_rows($1::text,$2::uuid)", name, admin)
		if err != nil {
			return nil, fmt.Errorf("export dataset: %w", err)
		}
		entry, err := writeRows(rows, name, writer)
		if err != nil {
			return nil, err
		}
		manifest = append(manifest, entry)
		writer.write("\n]")
	}
	writer.write("\n},\n\"graph\":{")
	for index, kind := range []string{"vertices", "edges"} {
		if index > 0 {
			writer.write(",")
		}
		writer.value(kind)
		writer.write(":[\n")
		rows, err := tx.Query(ctx, "SELECT directory.backup_graph_rows($1::text)", kind)
		if err != nil {
			return nil, fmt.Errorf("export graph: %w", err)
		}
		entry, err := writeRows(rows, "graph."+kind, writer)
		if err != nil {
			return nil, err
		}
		manifest = append(manifest, entry)
		writer.write("\n]")
	}
	writer.write("\n},\n\"manifest\":")
	writer.value(manifest)
	writer.write("\n}\n")
	if writer.err != nil {
		return nil, writer.err
	}
	if err = buffer.Flush(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return file, nil
}

func writeRows(rows pgx.Rows, name string, writer *jsonWriter) (Manifest, error) {
	defer rows.Close()
	entry := Manifest{Dataset: Dataset{Name: name}}
	hash := sha256.New()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return entry, err
		}
		canonical, err := canonicalRow(raw)
		if err != nil {
			return entry, err
		}
		if entry.Count > 0 {
			writer.write(",\n")
		}
		writer.write(string(canonical))
		if writer.err != nil {
			return entry, writer.err
		}
		if _, err = hash.Write(append(canonical, '\n')); err != nil {
			return entry, err
		}
		entry.Count++
	}
	entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return entry, rows.Err()
}

func buildRevision() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "unknown"
}
