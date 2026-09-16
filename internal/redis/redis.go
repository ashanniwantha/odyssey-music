package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	RedisHost     string
	RedisPort     int
	RedisPassword string
	DB            int
	PingTimeout   time.Duration
}

func NewClient(ctx context.Context, cfg RedisConfig) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.RedisPassword,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("unable to ping redis: %w", err)
	}

	return rdb, nil
}
