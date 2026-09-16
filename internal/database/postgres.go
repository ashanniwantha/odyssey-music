package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	DSN               string
	MaxConns          int
	MinConns          int
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
	PingTimeout       time.Duration
}

func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("unable to parse pool config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxConns)            // maximum number of connections
	poolConfig.MinConns = int32(cfg.MinConns)            // keep at least this many open
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime     // recycle connections older than this
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime     // close connections idle for this long
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod // background health checks
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}
	return pool, nil
}
