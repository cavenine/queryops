package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PubSub wraps Watermill SQL publisher and subscriber.
type PubSub struct {
	pool      *pgxpool.Pool
	publisher *sql.Publisher
	logger    watermill.LoggerAdapter
	cfg       *Config
}

// Config holds configuration for the pub/sub system.
type Config struct {
	// AutoInitializeSchema creates Watermill tables if they don't exist.
	//
	// In production, prefer setting this to false and using explicit migrations.
	AutoInitializeSchema bool

	// SubscriberPollInterval is the interval to wait between subsequent SELECT
	// queries if no messages are found.
	SubscriberPollInterval time.Duration
}

// DefaultConfig returns sensible defaults for development.
func DefaultConfig() *Config {
	return &Config{
		AutoInitializeSchema:   true,
		SubscriberPollInterval: 100 * time.Millisecond,
	}
}

// New creates a new PubSub instance.
func New(ctx context.Context, pool *pgxpool.Pool, cfg *Config) (*PubSub, error) {
	_ = ctx

	if pool == nil {
		return nil, fmt.Errorf("pool is nil")
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}

	logger := NewSlogAdapter(slog.Default())
	beginner := sql.BeginnerFromPgx(pool)

	publisher, err := sql.NewPublisher(
		beginner,
		sql.PublisherConfig{
			SchemaAdapter:        PostgreSQLSchema{},
			AutoInitializeSchema: cfg.AutoInitializeSchema,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("creating publisher: %w", err)
	}

	return &PubSub{
		pool:      pool,
		publisher: publisher,
		logger:    logger,
		cfg:       cfg,
	}, nil
}

// Publisher returns the Watermill publisher for sending messages.
func (ps *PubSub) Publisher() *sql.Publisher {
	return ps.publisher
}

// NewSubscriber creates a new subscriber for consuming messages.
// Each SSE connection should create its own subscriber.
func (ps *PubSub) NewSubscriber(ctx context.Context) (*sql.Subscriber, error) {
	_ = ctx

	if ps.pool == nil {
		return nil, fmt.Errorf("pool is nil")
	}

	beginner := sql.BeginnerFromPgx(ps.pool)

	config := ps.cfg
	if config == nil {
		config = DefaultConfig()
	}

	subscriber, err := sql.NewSubscriber(
		beginner,
		sql.SubscriberConfig{
			ConsumerGroup:    uuid.NewString(),
			PollInterval:     config.SubscriberPollInterval,
			ResendInterval:   time.Second,
			RetryInterval:    time.Second,
			SchemaAdapter:    PostgreSQLSchema{},
			OffsetsAdapter:   PostgreSQLOffsetsAdapter{},
			InitializeSchema: config.AutoInitializeSchema,
		},
		ps.logger,
	)
	if err != nil {
		return nil, fmt.Errorf("creating subscriber: %w", err)
	}

	return subscriber, nil
}

// Close shuts down the pub/sub system.
func (ps *PubSub) Close() error {
	if ps.publisher == nil {
		return nil
	}
	return ps.publisher.Close()
}
