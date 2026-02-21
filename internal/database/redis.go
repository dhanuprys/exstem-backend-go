package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
)

// NewRedisClient creates and validates a Redis client connection.
func NewRedisClient(ctx context.Context, cfg *config.Config, log zerolog.Logger) (*redis.Client, error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	rdb := redis.NewClient(opt)

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info().
		Str("addr", opt.Addr).
		Int("db", opt.DB).
		Msg("Redis connected")

	return rdb, nil
}
