package databasebackup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func rollback(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}

func sourceAdmin(ctx context.Context, tx pgx.Tx) (string, error) {
	var ids []string
	if err := tx.QueryRow(ctx, "SELECT array_agg(id::text) FROM identity.users WHERE role='SYS_ADMIN'").Scan(&ids); err != nil {
		return "", fmt.Errorf("read backup administrator: %w", err)
	}
	if len(ids) != 1 {
		return "", ErrTarget
	}
	return ids[0], nil
}

func targetIssues(ctx context.Context, tx pgx.Tx, actor string) ([]Issue, error) {
	rows, err := tx.Query(ctx, "SELECT directory.backup_target_issues($1::uuid)", actor)
	if err != nil {
		return nil, fmt.Errorf("inspect restore target: %w", err)
	}
	defer rows.Close()
	issues := []Issue{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		issues = append(issues, Issue{Code: code})
	}
	return issues, rows.Err()
}

type columnInventory map[string]map[string]string

func columns(ctx context.Context, tx pgx.Tx) (columnInventory, error) {
	rows, err := tx.Query(ctx, "SELECT dataset,column_name,type_name FROM directory.backup_columns()")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	inventory := columnInventory{}
	for rows.Next() {
		var table, column, typeName string
		if err := rows.Scan(&table, &column, &typeName); err != nil {
			return nil, err
		}
		if inventory[table] == nil {
			inventory[table] = map[string]string{}
		}
		inventory[table][column] = typeName
	}
	return inventory, rows.Err()
}

func (service *Service) Restore(ctx context.Context, file *os.File, actor, checksum string) (Restoration, error) {
	return service.RestoreWithProgress(ctx, file, actor, checksum, nil)
}

// RestoreWithProgress keeps all mutations in one transaction. The committing
// notification is the last cancellation gate; a commit error has an unknown outcome.
func (service *Service) RestoreWithProgress(ctx context.Context, file *os.File, actor, checksum string, report func(RestoreProgress) error) (result Restoration, resultErr error) {
	progress := RestoreProgress{Stage: "validating"}
	notify := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if report != nil {
			return report(progress)
		}
		return nil
	}
	if err := notify(); err != nil {
		return Restoration{}, err
	}
	preview, doc, err := service.inspect(ctx, file, actor)
	if err != nil {
		return Restoration{}, err
	}
	if checksum != preview.SHA256 {
		return Restoration{}, ErrInvalid
	}
	if !preview.CanRestore {
		return Restoration{}, ErrTarget
	}
	for _, dataset := range preview.Datasets {
		progress.Total += dataset.Count
	}
	progress.Total += preview.Graph.Vertices + preview.Graph.Edges
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Restoration{}, err
	}
	committed, commitAttempted := false, false
	defer func() {
		if committed {
			return
		}
		progress.Stage, progress.Dataset = "rolling_back", ""
		if report != nil {
			_ = report(progress)
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		rollbackErr := tx.Rollback(cleanup)
		if commitAttempted || rollbackErr != nil {
			resultErr = errors.Join(ErrOutcomeUnknown, resultErr, rollbackErr)
		}
	}()
	var sourceID, targetID pgtype.UUID
	if err = sourceID.Scan(doc.header.ExcludedSystemAdminID); err != nil {
		return Restoration{}, ErrInvalid
	}
	if err = targetID.Scan(actor); err != nil {
		return Restoration{}, ErrInvalid
	}
	queries := dbgen.New(tx)
	if err = queries.BeginDatabaseRestore(ctx, targetID); err != nil {
		return Restoration{}, fmt.Errorf("begin database restore: %w", err)
	}
	progress.Stage = "restoring"
	for _, name := range append(append([]string{}, tablesForSchema(doc.header.SchemaVersion)...), "graph.vertices", "graph.edges") {
		progress.Dataset = name
		if err = notify(); err != nil {
			return Restoration{}, err
		}
		if _, err = file.Seek(doc.offsets[name], io.SeekStart); err != nil {
			return Restoration{}, err
		}
		_, err = readRows(json.NewDecoder(contextReader{ctx, file}), name, func(dataset string, row json.RawMessage) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if kind, ok := strings.CutPrefix(dataset, "graph."); ok {
				err = queries.RestoreDatabaseGraphRow(ctx, dbgen.RestoreDatabaseGraphRowParams{Kind: kind, RowData: row})
			} else {
				converted, conversionErr := doc.convertRow(dataset, row)
				if conversionErr != nil {
					return conversionErr
				}
				err = queries.RestoreDatabaseRow(ctx, dbgen.RestoreDatabaseRowParams{Dataset: dataset, RowData: converted, SourceAdminID: sourceID, TargetAdminID: targetID})
			}
			if err != nil {
				return err
			}
			progress.Completed++
			return notify()
		})
		if err != nil {
			return Restoration{}, fmt.Errorf("restore dataset %s: %w", name, err)
		}
	}
	progress.Stage, progress.Dataset = "checking", ""
	if err = notify(); err != nil {
		return Restoration{}, err
	}
	if err = queries.FinishDatabaseRestore(ctx); err != nil {
		return Restoration{}, fmt.Errorf("validate database restore: %w", err)
	}
	progress.Stage = "committing"
	if err = notify(); err != nil {
		return Restoration{}, err
	}
	commitAttempted = true
	if err = tx.Commit(ctx); err != nil {
		return Restoration{}, fmt.Errorf("commit database restore: %w", err)
	}
	committed = true
	return Restoration{Status: "restored", SHA256: checksum, Datasets: preview.Datasets, Graph: preview.Graph, AdminMapping: AdminMapping{SourceID: doc.header.ExcludedSystemAdminID, TargetID: actor}}, nil
}
