package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/cavenine/queryops/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("database url is empty; set DATABASE_URL")
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
	if cfg.DatabaseMaxIdleMs > 0 {
		poolConfig.MaxConnIdleTime = time.Duration(cfg.DatabaseMaxIdleMs) * time.Millisecond
	} else if cfg.DatabaseMaxIdle > 0 {
		// Backward-compatible fallback. DATABASE_MAX_IDLE is deprecated.
		poolConfig.MaxConnIdleTime = time.Duration(cfg.DatabaseMaxIdle) * time.Millisecond
	}
	if cfg.DatabaseMaxConnLifeMs > 0 {
		poolConfig.MaxConnLifetime = time.Duration(cfg.DatabaseMaxConnLifeMs) * time.Millisecond
	} else if cfg.DatabaseMaxLifeMs > 0 {
		// Backward-compatible fallback for historical config.
		poolConfig.MaxConnLifetime = time.Duration(cfg.DatabaseMaxLifeMs) * time.Millisecond
	}

	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = map[string]string{}
	}
	if cfg.DatabaseStatementTimeoutMs > 0 {
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.DatabaseStatementTimeoutMs, 10)
	}
	if cfg.DatabaseIdleInTxTimeoutMs > 0 {
		poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = strconv.FormatInt(cfg.DatabaseIdleInTxTimeoutMs, 10)
	}
	if cfg.DatabaseAppName != "" {
		poolConfig.ConnConfig.RuntimeParams["application_name"] = cfg.DatabaseAppName
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	slog.DebugContext(ctx, "database pool initialized",
		"min_conns", poolConfig.MinConns,
		"max_conns", poolConfig.MaxConns,
		"max_conn_idle_time", poolConfig.MaxConnIdleTime.String(),
		"max_conn_lifetime", poolConfig.MaxConnLifetime.String(),
		"statement_timeout_ms", cfg.DatabaseStatementTimeoutMs,
		"idle_in_tx_timeout_ms", cfg.DatabaseIdleInTxTimeoutMs,
		"application_name", cfg.DatabaseAppName,
	)

	return pool, nil
}
