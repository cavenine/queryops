package pubsub

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	nc "github.com/nats-io/nats.go"

	wmnats "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
)

// PubSub provides publish/subscribe messaging backed by NATS.
//
// If no NATS URL is provided, an embedded NATS server is started automatically.
// This allows the application to run standalone on a single VPS without external
// dependencies, while still supporting external NATS for scaled deployments.
type PubSub struct {
	conn      *nc.Conn
	embedded  *EmbeddedServer // nil if using external NATS
	publisher message.Publisher
	logger    watermill.LoggerAdapter
}

// Config holds configuration for the pub/sub system.
type Config struct {
	// NATSUrl is the URL of the NATS server to connect to.
	// If empty, an embedded NATS server will be started.
	NATSUrl string
}

// New creates a new PubSub instance backed by NATS.
//
// If cfg.NATSUrl is empty, starts an embedded NATS server.
// Otherwise, connects to the external NATS server at the provided URL.
func New(ctx context.Context, cfg *Config) (*PubSub, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	conn, embedded, err := Connect(ctx, cfg.NATSUrl)
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS: %w", err)
	}

	logger := watermill.NewSlogLogger(slog.Default())

	// Create publisher using existing connection
	// JetStream is disabled for core NATS pub/sub
	pubConfig := wmnats.PublisherPublishConfig{
		Marshaler:         &wmnats.NATSMarshaler{},
		SubjectCalculator: wmnats.DefaultSubjectCalculator,
		JetStream:         wmnats.JetStreamConfig{Disabled: true},
	}

	publisher, err := wmnats.NewPublisherWithNatsConn(conn, pubConfig, logger)
	if err != nil {
		if embedded != nil {
			embedded.Shutdown()
		}
		conn.Close()
		return nil, fmt.Errorf("creating NATS publisher: %w", err)
	}

	return &PubSub{
		conn:      conn,
		embedded:  embedded,
		publisher: publisher,
		logger:    logger,
	}, nil
}

// Publisher returns the Watermill publisher for sending messages.
func (ps *PubSub) Publisher() message.Publisher {
	return ps.publisher
}

// NewSubscriber creates a new subscriber for consuming messages.
//
// Each SSE connection should create its own subscriber to receive all messages
// (fan-out pattern). Subscribers are ephemeral and should be closed when the
// SSE connection ends.
func (ps *PubSub) NewSubscriber(_ context.Context) (message.Subscriber, error) {
	// Create subscriber using existing connection
	// JetStream is disabled for core NATS pub/sub
	// No QueueGroupPrefix means each subscriber gets all messages (fan-out)
	subConfig := wmnats.SubscriberSubscriptionConfig{
		Unmarshaler:       &wmnats.NATSMarshaler{},
		SubjectCalculator: wmnats.DefaultSubjectCalculator,
		JetStream:         wmnats.JetStreamConfig{Disabled: true},
		// Empty QueueGroupPrefix = no queue group = fan-out to all subscribers
		QueueGroupPrefix: "",
	}

	subscriber, err := wmnats.NewSubscriberWithNatsConn(ps.conn, subConfig, ps.logger)
	if err != nil {
		return nil, fmt.Errorf("creating NATS subscriber: %w", err)
	}

	return subscriber, nil
}

// Close shuts down the pub/sub system.
//
// Closes the publisher, NATS connection, and embedded server (if running).
func (ps *PubSub) Close() error {
	var errs []error

	if ps.publisher != nil {
		if err := ps.publisher.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing publisher: %w", err))
		}
	}

	if ps.conn != nil {
		ps.conn.Close()
	}

	if ps.embedded != nil {
		ps.embedded.Shutdown()
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
