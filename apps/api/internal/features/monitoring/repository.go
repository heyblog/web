package monitoring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrStaleGeneration = errors.New("monitoring address generation changed")
	ErrInvalidRound    = errors.New("invalid monitoring round")
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (service *Service) Record(ctx context.Context, round Round) error {
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin monitoring round: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err := RecordTx(ctx, tx, round); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RecordTx permits atomic completion with the task's lease check. The site lock
// serializes address changes and concurrent rounds; retries do not add samples.
func RecordTx(ctx context.Context, tx pgx.Tx, round Round) error {
	var siteID pgtype.UUID
	if err := siteID.Scan(round.SiteID); err != nil || !siteID.Valid || round.Generation < 1 || strings.TrimSpace(round.ID) == "" || round.StartedAt.IsZero() || !round.CN.valid() || !round.GL.valid() {
		return ErrInvalidRound
	}
	queries := dbgen.New(tx)
	site, err := queries.LockSiteByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("lock monitoring site: %w", err)
	}
	if site.MonitoringGeneration != round.Generation {
		return ErrStaleGeneration
	}
	inserted, err := queries.InsertMonitoringRound(ctx, dbgen.InsertMonitoringRoundParams{SiteID: siteID, Generation: round.Generation, RoundID: round.ID, RoundAt: pgtype.Timestamptz{Time: round.StartedAt, Valid: true}, CnOutcome: string(round.CN), GlOutcome: string(round.GL)})
	if err != nil {
		return fmt.Errorf("insert monitoring round: %w", err)
	}
	if inserted == 0 {
		return nil
	}
	health, err := queries.ListMonitoringHealthWindow(ctx, dbgen.ListMonitoringHealthWindowParams{SiteID: siteID, Generation: round.Generation})
	if err != nil {
		return fmt.Errorf("read health window: %w", err)
	}
	scope, err := queries.ListMonitoringScopeWindow(ctx, dbgen.ListMonitoringScopeWindowParams{SiteID: siteID, Generation: round.Generation})
	if err != nil {
		return fmt.Errorf("read scope window: %w", err)
	}
	status, access := Summarize(rounds(health), rounds(scope))
	return queries.UpdateMonitoringSummary(ctx, dbgen.UpdateMonitoringSummaryParams{SiteID: siteID, Generation: round.Generation, Status: status, AccessScope: access, CheckedAt: pgtype.Timestamptz{Time: round.StartedAt, Valid: true}})
}

func rounds(rows []dbgen.DirectorySiteMonitoringRound) []Round {
	result := make([]Round, len(rows))
	for i, row := range rows {
		result[i] = Round{CN: Outcome(row.CnOutcome), GL: Outcome(row.GlOutcome)}
	}
	return result
}
