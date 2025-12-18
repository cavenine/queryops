package background

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/db"
)

// ClientConfig configures River queues for a client.
type ClientConfig struct {
	Queues map[string]river.QueueConfig
}

// DefaultClientConfig returns a sensible default client configuration.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: 10,
			},
		},
	}
}

type SortArgs struct {
	Strings []string
}

func (s SortArgs) Kind() string {
	return "sorter"
}

type SortWorker struct {
	// An embedded WorkerDefaults sets up default methods to fulfill the rest of
	// the Worker interface:
	river.WorkerDefaults[SortArgs]
}

func (w *SortWorker) Work(ctx context.Context, job *river.Job[SortArgs]) error {
	sort.Strings(job.Args.Strings)
	fmt.Printf("Sorted strings: %+v\n", job.Args.Strings)
	return nil
}

// NewWorkers constructs a Workers bundle and registers all workers.
// New workers should be added here.
func NewWorkers() *river.Workers {
	workers := river.NewWorkers()
	// TODO: register workers with river.AddWorker(workers, &YourWorker{})
	river.AddWorker(workers, &SortWorker{})
	return workers
}

// NewClient constructs a River client using the provided pool, workers, and config.
func NewClient(pool *pgxpool.Pool, workers *river.Workers, cfg *ClientConfig) (*river.Client[pgx.Tx], error) {
	if pool == nil {
		return nil, fmt.Errorf("nil pool provided to NewClient")
	}
	if cfg == nil {
		cfg = DefaultClientConfig()
	}

	riverCfg := &river.Config{
		Queues:  cfg.Queues,
		Workers: workers,
	}

	client, err := river.NewClient(riverpgxv5.New(pool), riverCfg)
	if err != nil {
		return nil, fmt.Errorf("creating river client: %w", err)
	}

	return client, nil
}

// MigrateRiver runs River's migrations against the configured database.
// It always migrates to River's latest schema version.
func MigrateRiver(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("creating pool for river migrations: %w", err)
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("creating river migrator: %w", err)
	}

	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("running river migrations: %w", err)
	}

	slog.Info("river migrations applied")
	return nil
}

// RunWorker starts a River client and works jobs until the context is cancelled.
// It is intended for use by the dedicated worker command.
func RunWorker(ctx context.Context, pool *pgxpool.Pool, cfg *ClientConfig) error {
	workers := NewWorkers()

	client, err := NewClient(pool, workers, cfg)
	if err != nil {
		return err
	}

	slog.Info("starting river workers")
	defer slog.Info("river workers stopped")

	if err := client.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("running river workers: %w", err)
	}

	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stopping river workers: %w", err)
	}

	return nil
}
