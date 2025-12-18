package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cavenine/queryops/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database url is empty; set DATABASE_URL")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}

	if cfg.DatabaseMinConns > 0 {
		poolConfig.MinConns = cfg.DatabaseMinConns
	}
	if cfg.DatabaseMaxConns > 0 {
		poolConfig.MaxConns = cfg.DatabaseMaxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	slog.Debug("database pool initialized")

	return pool, nil
}
