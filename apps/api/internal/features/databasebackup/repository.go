package databasebackup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func rollback(ctx context.Context, tx pgx.Tx) { _ = tx.Rollback(ctx) }

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
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Restoration{}, err
	}
	defer rollback(ctx, tx)
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
	for _, name := range append(append([]string{}, tableNames...), "graph.vertices", "graph.edges") {
		if _, err = file.Seek(doc.offsets[name], io.SeekStart); err != nil {
			return Restoration{}, err
		}
		_, err = readRows(json.NewDecoder(file), name, func(dataset string, row json.RawMessage) error {
			if kind, ok := strings.CutPrefix(dataset, "graph."); ok {
				return queries.RestoreDatabaseGraphRow(ctx, dbgen.RestoreDatabaseGraphRowParams{Kind: kind, RowData: row})
			}
			return queries.RestoreDatabaseRow(ctx, dbgen.RestoreDatabaseRowParams{Dataset: dataset, RowData: row, SourceAdminID: sourceID, TargetAdminID: targetID})
		})
		if err != nil {
			return Restoration{}, fmt.Errorf("restore dataset %s: %w", name, err)
		}
	}
	if err = queries.FinishDatabaseRestore(ctx); err != nil {
		return Restoration{}, fmt.Errorf("validate database restore: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Restoration{}, fmt.Errorf("commit database restore: %w", err)
	}
	return Restoration{Status: "restored", SHA256: checksum, Datasets: preview.Datasets, Graph: preview.Graph, AdminMapping: AdminMapping{SourceID: doc.header.ExcludedSystemAdminID, TargetID: actor}}, nil
}
