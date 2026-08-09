package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/config"
)

func redisOptions(input config.RedisConfig) (*redis.Options, error) {
	options, err := redis.ParseURL(input.URL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}

	options.DialTimeout = input.DialTimeout
	options.ReadTimeout = input.ReadTimeout
	options.WriteTimeout = input.WriteTimeout

	return options, nil
}

func OpenRedis(ctx context.Context, input config.RedisConfig) (*redis.Client, error) {
	options, err := redisOptions(input)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	return client, nil
}
