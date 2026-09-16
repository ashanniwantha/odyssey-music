package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ashanniwantha/odyssey-music/internal/config"
	"github.com/ashanniwantha/odyssey-music/internal/database"
	"github.com/ashanniwantha/odyssey-music/internal/logger"
	"github.com/ashanniwantha/odyssey-music/internal/redis"
	"github.com/go-chi/chi"
)

func main() {
	// load configs
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configurations: %v", err)
		os.Exit(1)
	}

	// Intialize the logger
	log := logger.New(cfg.AppEnv)
	log.Info("Starting application", "env", cfg.AppEnv, "port", cfg.AppPort)
	fmt.Printf("Database URL target: postgres://%s:***@%s:%d/%s\n",
		cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	// Database pool
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	dbCfg := database.PoolConfig{
		DSN:               dsn,
		MaxConns:          cfg.DBMaxConns,
		MinConns:          cfg.DBMinConns,
		MaxConnLifetime:   cfg.DBMaxConnLifetime,
		MaxConnIdleTime:   cfg.DBMaxConnIdleTime,
		HealthCheckPeriod: cfg.DBHealthCheckPeriod,
		ConnectTimeout:    cfg.DBConnectTimeout,
		PingTimeout:       cfg.DBPingTimeout,
	}

	pool, err := database.NewPool(context.Background(), dbCfg)
	if err != nil {
		log.Error("failed to intialize database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	log.Info("Database connected", "mode", cfg.AppEnv)

	redisCfg := redis.RedisConfig{
		RedisHost:     cfg.RedisHost,
		RedisPort:     cfg.RedisPort,
		RedisPassword: cfg.RedisPassword,
		DB:            cfg.RedisDB,
		PingTimeout:   cfg.RedisPingTimeout,
	}

	rdb, err := redis.NewClient(context.Background(), redisCfg)
	if err != nil {
		log.Error("failed to initialize redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// Chi Router
	r := chi.NewRouter()

	// HTTP server with timeouts
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AppPort),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		log.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("shutting down", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", "err", err)
	}

	log.Info("server stopped")
}
