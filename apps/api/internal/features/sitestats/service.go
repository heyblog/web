package sitestats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"heyblog-api/internal/domain/site"
	"heyblog-api/internal/features/publicview"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"
)

type Event struct {
	EventID string `json:"eventId" format:"uuid"`
	ShortID string `json:"shortId" pattern:"^[0-9A-Za-z]{9}$"`
}

type Store interface {
	IncrementSiteDisplayMetrics(context.Context, dbgen.IncrementSiteDisplayMetricsParams) error
	GetSiteMetrics(context.Context, []string) ([]dbgen.GetSiteMetricsRow, error)
	GetSiteOutboundTarget(context.Context, string) (dbgen.GetSiteOutboundTargetRow, error)
	RecordSiteMetricEvents(context.Context, dbgen.RecordSiteMetricEventsParams) error
}

type Service struct {
	store  Store
	logger *slog.Logger
}

func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: dbgen.New(pool), logger: logger}
}

func (service *Service) RecordQuery(ctx context.Context, ids []string) {
	service.increment(ctx, ids, 1, 0)
}

func (service *Service) RecordResponse(ctx context.Context, ids []string) map[string]publicview.SiteMetrics {
	service.increment(ctx, ids, 0, 1)
	counts := make(map[string]publicview.SiteMetrics, len(ids))
	if len(ids) == 0 {
		return counts
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	rows, err := service.store.GetSiteMetrics(bounded, ids)
	if err != nil {
		service.report(ctx, err, "read")
		return counts
	}
	for _, row := range rows {
		counts[row.ShortID] = publicview.SiteMetrics{ClickCount: row.ClickCount, ImpressionCount: row.ImpressionCount, ResponseCount: row.ResponseCount, QueryCount: row.QueryCount}
	}
	return counts
}

func (service *Service) increment(ctx context.Context, ids []string, queries, responses int64) {
	if len(ids) == 0 {
		return
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := service.store.IncrementSiteDisplayMetrics(bounded, dbgen.IncrementSiteDisplayMetricsParams{ShortIds: ids, QueryIncrement: queries, ResponseIncrement: responses}); err != nil {
		service.report(ctx, err, "increment")
	}
}

func (service *Service) Outbound(ctx context.Context, shortID string) (string, error) {
	if err := site.ValidateShortID(shortID); err != nil {
		return "", badInput()
	}
	row, err := service.store.GetSiteOutboundTarget(ctx, shortID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.New(apperror.KindNotFound, "not_found", "site was not found")
	}
	if err != nil {
		return "", unavailable(err)
	}
	target, err := (site.Address{Scheme: row.Scheme, NormalizedHost: row.NormalizedHost, BasePath: row.BasePath}).HomepageURL()
	if err != nil {
		return "", unavailable(err)
	}
	return target, nil
}

func (service *Service) RecordEvents(ctx context.Context, events []Event, kind string) error {
	if len(events) == 0 || len(events) > 100 || kind != "CLICK" && kind != "IMPRESSION" {
		return badInput()
	}
	ids := make([]pgtype.UUID, len(events))
	sites := make([]string, len(events))
	for i, event := range events {
		if err := site.ValidateShortID(event.ShortID); err != nil {
			return badInput()
		}
		if err := ids[i].Scan(event.EventID); err != nil || !ids[i].Valid {
			return badInput()
		}
		sites[i] = event.ShortID
	}
	if err := service.store.RecordSiteMetricEvents(ctx, dbgen.RecordSiteMetricEventsParams{EventIds: ids, ShortIds: sites, Kind: kind}); err != nil {
		return unavailable(err)
	}
	return nil
}

func (service *Service) report(ctx context.Context, err error, operation string) {
	service.logger.WarnContext(ctx, "site metric operation failed", "event", "site_metric_failed", "operation", operation, "error_type", fmt.Sprintf("%T", err))
}

func badInput() error {
	return apperror.New(apperror.KindBadRequest, "invalid_site_metric", "site metric is invalid")
}
func unavailable(err error) error {
	return apperror.Wrap(err, apperror.KindUnavailable, "service_unavailable", "site metrics are temporarily unavailable", "site metrics")
}
