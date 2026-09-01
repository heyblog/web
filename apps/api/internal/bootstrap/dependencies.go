package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/cache"
	"heyblog-api/internal/config"
	"heyblog-api/internal/database"
	dbgen "heyblog-api/internal/database/gen"
	"heyblog-api/internal/mail"
)

type Dependencies struct {
	Database           *pgxpool.Pool
	Redis              *redis.Client
	MailSender         mail.Sender
	VerificationMailer *mail.VerificationMailer
	views              publicview.Reader

	pingDatabase  func(context.Context) error
	pingRedis     func(context.Context) error
	closeDatabase func()
	closeRedis    func() error
	closeOnce     sync.Once
	closeErr      error
}

type dependencyOperations struct {
	migrate       func(context.Context, string) error
	openDatabase  func(context.Context, config.DatabaseConfig) (*pgxpool.Pool, error)
	closeDatabase func(*pgxpool.Pool)
	openRedis     func(context.Context, config.RedisConfig) (*redis.Client, error)
	closeRedis    func(*redis.Client) error
	openMail      func(context.Context, config.MailConfig) (mail.Sender, error)
}

type ReadinessError struct {
	component string
	cause     error
}

func Open(ctx context.Context, configuration config.Config) (*Dependencies, error) {
	return open(ctx, configuration, dependencyOperations{
		migrate:       database.Migrate,
		openDatabase:  database.OpenPool,
		closeDatabase: func(pool *pgxpool.Pool) { pool.Close() },
		openRedis:     cache.OpenRedis,
		closeRedis:    func(client *redis.Client) error { return client.Close() },
		openMail: func(ctx context.Context, configuration config.MailConfig) (mail.Sender, error) {
			return mail.OpenSES(ctx, configuration.SES.Region)
		},
	})
}

func open(ctx context.Context, configuration config.Config, operations dependencyOperations) (*Dependencies, error) {
	if err := operations.migrate(ctx, configuration.MigrationDatabaseURL); err != nil {
		return nil, withStage("database_migration", err)
	}

	pool, err := operations.openDatabase(ctx, configuration.Database)
	if err != nil {
		return nil, withStage("database_open", err)
	}

	redisClient, err := operations.openRedis(ctx, configuration.Redis)
	if err != nil {
		operations.closeDatabase(pool)
		return nil, withStage("redis_open", err)
	}

	mailSender, err := operations.openMail(ctx, configuration.Mail)
	if err != nil {
		closeErr := operations.closeRedis(redisClient)
		operations.closeDatabase(pool)
		return nil, errors.Join(
			withStage("mail_open", err),
			withStage("redis_close", closeErr),
		)
	}

	return &Dependencies{
		Database:           pool,
		Redis:              redisClient,
		MailSender:         mailSender,
		VerificationMailer: mail.NewVerificationMailer(mailSender, configuration.Mail.Senders.Verification.Address),
		views:              publicview.New(dbgen.New(pool)),
		pingDatabase: func(ctx context.Context) error {
			return pool.Ping(ctx)
		},
		pingRedis: func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		},
		closeDatabase: func() {
			operations.closeDatabase(pool)
		},
		closeRedis: func() error { return operations.closeRedis(redisClient) },
	}, nil
}

func (dependencies *Dependencies) PublicViews() publicview.Reader {
	return dependencies.views
}

func (dependencies *Dependencies) Ready(ctx context.Context) error {
	checks := []struct {
		component string
		check     func(context.Context) error
	}{
		{component: "database", check: dependencies.pingDatabase},
		{component: "redis", check: dependencies.pingRedis},
	}
	errorsChannel := make(chan error, len(checks))
	for _, check := range checks {
		go func() {
			if check.check == nil {
				errorsChannel <- &ReadinessError{component: check.component, cause: errors.New("readiness check is not configured")}
				return
			}
			if err := check.check(ctx); err != nil {
				errorsChannel <- &ReadinessError{component: check.component, cause: err}
				return
			}
			errorsChannel <- nil
		}()
	}

	var readinessErrors []error
	for range checks {
		if err := <-errorsChannel; err != nil {
			readinessErrors = append(readinessErrors, err)
		}
	}
	return errors.Join(readinessErrors...)
}

func (dependencies *Dependencies) DatabasePool() *pgxpool.Pool {
	return dependencies.Database
}

func (dependencies *Dependencies) RedisClient() *redis.Client { return dependencies.Redis }

func (dependencies *Dependencies) Mail() mail.Sender { return dependencies.MailSender }

func (dependencies *Dependencies) Verification() *mail.VerificationMailer {
	return dependencies.VerificationMailer
}

func (dependencies *Dependencies) Close() error {
	dependencies.closeOnce.Do(func() {
		var closeErrors []error
		if dependencies.closeRedis != nil {
			if err := dependencies.closeRedis(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close Redis: %w", err))
			}
		}
		if dependencies.closeDatabase != nil {
			dependencies.closeDatabase()
		}
		dependencies.closeErr = errors.Join(closeErrors...)
	})
	return dependencies.closeErr
}

func (err *ReadinessError) Error() string {
	return err.component + " readiness: " + err.cause.Error()
}

func (err *ReadinessError) Unwrap() error {
	return err.cause
}

func (err *ReadinessError) Component() string {
	return err.component
}
