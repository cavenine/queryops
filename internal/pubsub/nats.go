package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// EmbeddedServer wraps an in-process NATS server.
type EmbeddedServer struct {
	server *server.Server
}

// StartEmbedded starts an in-process NATS server.
//
// The server binds to localhost only (not exposed externally) and uses
// an ephemeral port to avoid conflicts. Returns the server and its client URL.
func StartEmbedded(ctx context.Context) (*EmbeddedServer, string, error) {
	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1, // Ephemeral port
		NoLog:          false,
		NoSigs:         true, // Don't install signal handlers (let the app handle them)
		MaxControlLine: server.MAX_CONTROL_LINE_SIZE,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, "", fmt.Errorf("creating embedded NATS server: %w", err)
	}

	// Configure logging to use slog
	ns.SetLoggerV2(newNATSLogger(), false, false, false)

	go ns.Start()

	// Wait for server to be ready
	const maxWait = 5 * time.Second
	if !ns.ReadyForConnections(maxWait) {
		ns.Shutdown()
		return nil, "", fmt.Errorf("embedded NATS server failed to start within %v", maxWait)
	}

	clientURL := ns.ClientURL()
	slog.InfoContext(ctx, "embedded NATS server started", "url", clientURL)

	return &EmbeddedServer{server: ns}, clientURL, nil
}

// Shutdown gracefully stops the embedded NATS server.
func (e *EmbeddedServer) Shutdown() {
	if e.server != nil {
		e.server.Shutdown()
		e.server.WaitForShutdown()
	}
}

// Connect establishes a connection to NATS.
//
// If natsURL is empty, starts an embedded server and connects to it.
// If natsURL is provided, connects to the external server.
//
// Returns the connection, an optional embedded server (nil if using external),
// and any error.
func Connect(ctx context.Context, natsURL string) (*nats.Conn, *EmbeddedServer, error) {
	var embedded *EmbeddedServer
	var url string

	if natsURL == "" {
		var err error
		embedded, url, err = StartEmbedded(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("starting embedded NATS: %w", err)
		}
	} else {
		url = natsURL
		slog.InfoContext(ctx, "connecting to external NATS server", "url", url)
	}

	nc, err := nats.Connect(url,
		nats.Name("queryops"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1), // Unlimited reconnects
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				slog.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			slog.Error("NATS error", "error", err)
		}),
	)
	if err != nil {
		if embedded != nil {
			embedded.Shutdown()
		}
		return nil, nil, fmt.Errorf("connecting to NATS: %w", err)
	}

	return nc, embedded, nil
}

// natsLogger adapts NATS server logging to slog.
type natsLogger struct{}

func newNATSLogger() *natsLogger {
	return &natsLogger{}
}

func (l *natsLogger) Noticef(format string, v ...any) {
	slog.Info(fmt.Sprintf(format, v...), "component", "nats-server")
}

func (l *natsLogger) Warnf(format string, v ...any) {
	slog.Warn(fmt.Sprintf(format, v...), "component", "nats-server")
}

func (l *natsLogger) Fatalf(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...), "component", "nats-server")
}

func (l *natsLogger) Errorf(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...), "component", "nats-server")
}

func (l *natsLogger) Debugf(format string, v ...any) {
	slog.Debug(fmt.Sprintf(format, v...), "component", "nats-server")
}

func (l *natsLogger) Tracef(format string, v ...any) {
	slog.Debug(fmt.Sprintf(format, v...), "component", "nats-server")
}
