package taxonomy

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"heyblog-api/internal/features/auth"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"heyblog-api/internal/platform/apperror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Authenticator interface {
	Current(context.Context, *http.Request) (auth.User, error)
}

type Service struct {
	pool *pgxpool.Pool
	auth Authenticator
}

func NewService(pool *pgxpool.Pool, authService Authenticator) *Service {
	return &Service{pool: pool, auth: authService}
}
func (s *Service) authorize(ctx context.Context, request *http.Request) error {
	if request == nil || s.auth == nil {
		return apperror.New(apperror.KindUnauthorized, "unauthenticated", "authentication is required")
	}
	user, err := s.auth.Current(ctx, request)
	if err != nil {
		var ae *auth.AuthError
		if errors.As(err, &ae) {
			return apperror.Wrap(err, apperror.KindUnauthorized, ae.Code, "authentication is required", "authorize taxonomy")
		}
		return err
	}
	if user.Role != auth.RoleSysAdmin && (user.Role != auth.RoleAdmin || !slices.Contains(user.Permissions, auth.PermissionTaxonomyManage)) {
		return apperror.New(apperror.KindForbidden, "taxonomy_permission_required", "taxonomy management permission is required")
	}
	return nil
}

func (s *Service) transaction(ctx context.Context, fn func(pgx.Tx, graph) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = dbgen.New(tx).LockTaxonomy(ctx); err != nil {
		return databaseError(err)
	}
	state, err := readGraph(ctx, tx)
	if err != nil {
		return databaseError(err)
	}
	if err = fn(tx, state); err != nil {
		return databaseError(err)
	}
	return databaseError(tx.Commit(ctx))
}
func (s *Service) List(ctx context.Context) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(_ pgx.Tx, g graph) error { result = g.Catalog; return nil })
	return result, err
}
func (s *Service) Preview(ctx context.Context, input ChangeInput) (Preview, error) {
	var result Preview
	err := s.transaction(ctx, func(_ pgx.Tx, g graph) error {
		if err := checkRevision(g, input.ExpectedRevision); err != nil {
			return err
		}
		p, err := buildPlan(g, input)
		result = p.Preview
		return err
	})
	return result, err
}
func (s *Service) Apply(ctx context.Context, input ChangeInput) (Catalog, error) {
	var result Catalog
	err := s.transaction(ctx, func(tx pgx.Tx, g graph) error {
		if err := checkRevision(g, input.ExpectedRevision); err != nil {
			return err
		}
		p, err := buildPlan(g, input)
		if err != nil {
			return err
		}
		if len(p.Blockers) > 0 {
			return apperror.New(apperror.KindConflict, "taxonomy_change_blocked", "resolve the preview blockers before applying")
		}
		if input.Fingerprint == "" || input.Fingerprint != p.Fingerprint {
			return apperror.New(apperror.KindConflict, "taxonomy_preview_changed", "preview this change again before applying")
		}
		if err = applyPlan(ctx, tx, p); err != nil {
			return err
		}
		fresh, err := readGraph(ctx, tx)
		result = fresh.Catalog
		return err
	})
	return result, err
}
func checkRevision(g graph, expected string) error {
	if expected == "" || expected != g.Revision {
		return apperror.New(apperror.KindConflict, "taxonomy_revision_changed", "taxonomy changed; refresh and try again")
	}
	return nil
}
func databaseError(err error) error {
	if err == nil {
		return nil
	}
	var ae *apperror.Error
	if errors.As(err, &ae) {
		return err
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && (pe.Code == "23505" || pe.Code == "23503" || pe.Code == "23514" || pe.Code == "P0001") {
		return apperror.Wrap(err, apperror.KindConflict, "taxonomy_conflict", "the change conflicts with taxonomy or existing references", "write taxonomy")
	}
	return apperror.Wrap(err, apperror.KindInternal, "internal_error", "the taxonomy operation failed", "manage taxonomy")
}
