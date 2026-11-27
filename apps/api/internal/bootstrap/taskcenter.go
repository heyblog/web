package bootstrap

import (
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/taskcenter"
	"heyblog-api/internal/platform/config"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registerTaskCenter(api huma.API, pool *pgxpool.Pool, authentication *auth.Service, configuration config.Config) *taskcenter.Repository {
	repository := taskcenter.NewRepository(pool, time.Now)
	signing := taskcenter.NewSigning(repository, configuration.Edge.MasterKey)
	taskcenter.RegisterRoutes(api, taskcenter.Routes{Repository: repository, Signing: signing, Execution: taskcenter.NewExecution(repository), Auth: authentication, WebToken: configuration.WebToken, WorkerToken: configuration.Edge.WorkerToken})
	return repository
}
