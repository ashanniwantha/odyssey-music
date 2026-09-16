package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	// Safe parsing of the network layout DSN
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("unable to parse pool config: %w", err)
	}

	// Safely capture or allocate the nested ConnConfig to prevent panic risks
	if poolConfig.ConnConfig == nil {
		poolConfig.ConnConfig = &pgx.ConnConfig{}
	}

	// Apply your connection limit and timeout parameters explicitly
	poolConfig.MaxConns = int32(cfg.MaxConns)
	poolConfig.MinConns = int32(cfg.MinConns)
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	// Initialize pool lifecycle with Background context to decouple from request timeouts
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Check network availability using your isolated ping timeout parameter
	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close() // Gracefully clean up half-allocated connection threads
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}
