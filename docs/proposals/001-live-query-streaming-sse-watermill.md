# Proposal 001: Live Query Results Streaming via SSE with Watermill Pub/Sub

**Status:** Draft  
**Author:** AI Assistant  
**Created:** 2025-12-20  
**Related Issues:** TBD (will be created after proposal review)

---

## Executive Summary

This proposal introduces real-time query result streaming when users submit distributed queries to osquery hosts. Instead of the current 1-second polling mechanism, we'll implement a PostgreSQL-backed pub/sub system using [Watermill](https://watermill.io/) to push updates instantly when results arrive from osquery agents.

**Key Benefits:**
- Near-instant result delivery (sub-100ms vs 0-1000ms polling latency)
- Reduced database load (event-driven vs continuous polling)
- Foundation for future real-time features
- Better user experience for interactive query sessions

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Current Architecture](#2-current-architecture)
3. [Proposed Solution](#3-proposed-solution)
4. [Technical Design](#4-technical-design)
5. [Database Schema](#5-database-schema)
6. [API Changes](#6-api-changes)
7. [Implementation Phases](#7-implementation-phases)
8. [Testing Strategy](#8-testing-strategy)
9. [Rollout Plan](#9-rollout-plan)
10. [Alternatives Considered](#10-alternatives-considered)
11. [Open Questions](#11-open-questions)
12. [References](#12-references)

---

## 1. Problem Statement

### Current User Flow

1. User opens host details page (`/hosts/:id`)
2. User submits a query via the dialog (`POST /hosts/:id/query`)
3. Query is queued in `distributed_queries` table
4. User waits for osquery agent to poll, execute, and return results
5. Browser polls `GET /hosts/:id/results` every 1 second
6. Results appear after 0-N seconds depending on:
   - Agent poll interval (typically 10-60 seconds)
   - Network latency
   - Query execution time
   - **Plus** 0-1 second polling latency

### Problems

| Issue | Impact |
|-------|--------|
| **Polling latency** | 0-1 second delay between result arriving and user seeing it |
| **Database load** | Every connected browser executes `SELECT` every second |
| **Wasted queries** | 99%+ of polls return unchanged data |
| **Scalability** | N browsers × 1 query/sec = significant DB load |
| **User experience** | Results "pop in" randomly instead of appearing immediately |

### Success Criteria

- Results appear in browser within 100ms of `distributed_write` completion
- No polling required for result updates
- Database queries only occur when data changes
- System scales to 100+ concurrent viewers per host

---

## 2. Current Architecture

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              CURRENT FLOW                               │
└─────────────────────────────────────────────────────────────────────────┘

  Browser                    Web Server                    Database
     │                           │                            │
     │  POST /hosts/:id/query    │                            │
     │ ─────────────────────────►│                            │
     │                           │  INSERT distributed_query  │
     │                           │ ──────────────────────────►│
     │                           │                            │
     │  GET /hosts/:id/results   │                            │
     │ ─────────────────────────►│                            │
     │        (SSE open)         │                            │
     │                           │                            │
     │◄──────────────────────────│◄───────────────────────────│
     │    Initial results        │   SELECT query_results     │
     │                           │                            │
     │         ...               │         ...                │
     │    (1 sec passes)         │                            │
     │                           │                            │
     │                           │   SELECT query_results     │
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│◄───────────────────────────│
     │   (no change, skip)       │                            │
     │                           │                            │
     │         ...               │         ...                │
     │    (repeat forever)       │                            │


  osquery Agent              Web Server                    Database
     │                           │                            │
     │  POST /distributed_read   │                            │
     │ ─────────────────────────►│                            │
     │                           │  UPDATE targets → 'sent'   │
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│◄───────────────────────────│
     │   { queries: {...} }      │                            │
     │                           │                            │
     │   (executes query)        │                            │
     │                           │                            │
     │  POST /distributed_write  │                            │
     │ ─────────────────────────►│                            │
     │                           │  UPDATE targets → results  │
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│                            │
     │        200 OK             │                            │
```

### Key Files

| File | Purpose |
|------|---------|
| `features/osquery/handlers.go:392-447` | `RunQuery` - queues distributed query |
| `features/osquery/handlers.go:331-390` | `HostResultsSSE` - polling SSE handler |
| `features/osquery/handlers.go:205-276` | `DistributedWrite` - receives results from agent |
| `features/osquery/services/host_repository.go:256-269` | `SaveQueryResults` - persists results |
| `features/osquery/services/host_repository.go:279-305` | `GetRecentResults` - fetches for display |
| `features/osquery/pages/host_details.templ:53-99` | `HostResultsTable` - renders results table |

### Current SSE Implementation

```go
// features/osquery/handlers.go:331-390
func (h *Handlers) HostResultsSSE(w http.ResponseWriter, r *http.Request) {
    // ... validation ...
    
    sse := datastar.NewSSE(w, r)
    ticker := time.NewTicker(time.Second)  // <-- POLLING
    defer ticker.Stop()

    var last []byte
    for {
        results, err := h.repo.GetRecentResults(ctx, hostID)  // <-- DB QUERY
        // ...
        
        b, _ := json.Marshal(results)
        if !bytes.Equal(b, last) {  // <-- DIFF CHECK
            last = b
            sse.PatchElementTempl(pages.HostResultsTable(hostIDStr, results))
        }

        select {
        case <-ctx.Done():
            return
        case <-ticker.C:  // <-- WAIT 1 SECOND
        }
    }
}
```

---

## 3. Proposed Solution

### Overview

Replace the polling mechanism with a PostgreSQL-backed pub/sub system using Watermill. When osquery results arrive, publish an event. SSE handlers subscribe to events for their host and push updates immediately.

### Why Watermill?

| Criteria                  | Watermill SQL  | River (existing) | Redis Streams | NATS |
|---------------------------|----------------|------------------|---------------|------|
| **No new infra**          | Yes (Postgres) | Yes (Postgres)   | No            | No   |
| **Pub/Sub native**        | Yes            | No (job queue)   | Yes           | Yes  |
| **pgx support**           | Yes (v4+)      | N/A              | N/A           | N/A  |
| **Fan-out pattern**       | Yes            | No               | Yes           | Yes  |
| **Transactional publish** | Yes            | Yes              | No            | No   |
| **Complexity**            | Low            | Medium           | Medium        | High |

Watermill is purpose-built for pub/sub patterns and has first-class PostgreSQL support with pgx adapters, making it ideal for this use case.

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                             PROPOSED FLOW                               │
└─────────────────────────────────────────────────────────────────────────┘

  Browser                    Web Server                    Database
     │                           │                            │
     │  GET /hosts/:id/results   │                            │
     │ ─────────────────────────►│                            │
     │        (SSE open)         │                            │
     │                           │  SELECT initial results    │
     │◄──────────────────────────│◄───────────────────────────│
     │    Initial results        │                            │
     │                           │                            │
     │                           │  SUBSCRIBE query_results   │
     │                           │ ──────────────────────────►│
     │                           │        (waiting...)        │
     │                           │                            │


  osquery Agent              Web Server                    Database
     │                           │                            │
     │  POST /distributed_write  │                            │
     │ ─────────────────────────►│                            │
     │                           │  UPDATE targets → results  │
     │                           │ ──────────────────────────►│
     │                           │                            │
     │                           │  PUBLISH query_result_event│
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│                            │
     │        200 OK             │                            │


  Web Server (SSE Handler)                               Database
     │                                                      │
     │              (receives published event)              │
     │◄─────────────────────────────────────────────────────│
     │                                                      │
     │  SELECT updated results (optional)                   │
     │ ────────────────────────────────────────────────────►│
     │◄─────────────────────────────────────────────────────│
     │                                                      │
     │  PUSH to browser via SSE                             │
     │                                                      │


  Browser                    Web Server
     │                           │
     │◄──────────────────────────│
     │   SSE: Updated results    │
     │   (instant!)              │
```

### Topic Design

**Topic naming:** `query_results:{host_id}`

Each host gets its own topic. When a browser opens the host details page, it subscribes to that host's topic. When results arrive for any query on that host, all connected browsers receive the update.

**Rationale:**
- Matches UI subscription pattern (per-host details page)
- Simple to implement and reason about
- Efficient for typical use case (few viewers per host)

---

## 4. Technical Design

### 4.1 Package Structure

```
internal/
└── pubsub/
    ├── pubsub.go        # PubSub struct, New(), Close()
    ├── publisher.go     # Publisher wrapper and helpers
    ├── subscriber.go    # Subscriber wrapper and helpers
    ├── events.go        # Event type definitions
    ├── schema.go        # Custom PostgreSQL schema adapter (if needed)
    └── logger.go        # Watermill logger adapter for slog
```

### 4.2 Core Types

```go
// internal/pubsub/pubsub.go

package pubsub

import (
    "context"
    "fmt"

    "github.com/ThreeDotsLabs/watermill"
    "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
    "github.com/jackc/pgx/v5/pgxpool"
)

// PubSub wraps Watermill SQL publisher and subscriber.
type PubSub struct {
    pool       *pgxpool.Pool
    publisher  *sql.Publisher
    logger     watermill.LoggerAdapter
}

// Config holds configuration for the pub/sub system.
type Config struct {
    // AutoInitializeSchema creates Watermill tables if they don't exist.
    // Set to false in production if using explicit migrations.
    AutoInitializeSchema bool
}

// DefaultConfig returns sensible defaults for development.
func DefaultConfig() *Config {
    return &Config{
        AutoInitializeSchema: true,
    }
}

// New creates a new PubSub instance.
func New(ctx context.Context, pool *pgxpool.Pool, cfg *Config) (*PubSub, error) {
    if cfg == nil {
        cfg = DefaultConfig()
    }
    
    logger := NewSlogAdapter(slog.Default())
    
    publisher, err := sql.NewPublisher(
        sql.BeginnerFromPgxPool(pool),
        sql.PublisherConfig{
            SchemaAdapter:        sql.DefaultPostgreSQLSchema{},
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
    }, nil
}

// Publisher returns the Watermill publisher for sending messages.
func (ps *PubSub) Publisher() *sql.Publisher {
    return ps.publisher
}

// NewSubscriber creates a new subscriber for consuming messages.
// Each SSE connection should create its own subscriber.
func (ps *PubSub) NewSubscriber(ctx context.Context) (*sql.Subscriber, error) {
    return sql.NewSubscriber(
        sql.BeginnerFromPgxPool(ps.pool),
        sql.SubscriberConfig{
            SchemaAdapter:    sql.DefaultPostgreSQLSchema{},
            OffsetsAdapter:   sql.DefaultPostgreSQLOffsetsAdapter{},
            PollInterval:     100 * time.Millisecond, // Fast polling for low latency
            InitializeSchema: false, // Publisher handles this
        },
        ps.logger,
    )
}

// Close shuts down the pub/sub system.
func (ps *PubSub) Close() error {
    return ps.publisher.Close()
}
```

### 4.3 Event Types

```go
// internal/pubsub/events.go

package pubsub

import (
    "encoding/json"
    "time"

    "github.com/ThreeDotsLabs/watermill/message"
    "github.com/google/uuid"
)

// TopicQueryResults returns the topic name for a host's query results.
func TopicQueryResults(hostID uuid.UUID) string {
    return fmt.Sprintf("query_results:%s", hostID.String())
}

// QueryResultEvent is published when distributed query results are saved.
type QueryResultEvent struct {
    // HostID is the host that executed the query.
    HostID uuid.UUID `json:"host_id"`
    
    // QueryID is the distributed query ID.
    QueryID uuid.UUID `json:"query_id"`
    
    // Status is the result status: "completed" or "failed".
    Status string `json:"status"`
    
    // OccurredAt is when the result was saved.
    OccurredAt time.Time `json:"occurred_at"`
    
    // Error is set if status is "failed".
    Error *string `json:"error,omitempty"`
}

// ToMessage converts the event to a Watermill message.
func (e QueryResultEvent) ToMessage() *message.Message {
    payload, _ := json.Marshal(e)
    msg := message.NewMessage(uuid.New().String(), payload)
    msg.Metadata.Set("event_type", "query_result")
    msg.Metadata.Set("host_id", e.HostID.String())
    msg.Metadata.Set("query_id", e.QueryID.String())
    return msg
}

// ParseQueryResultEvent parses a Watermill message into a QueryResultEvent.
func ParseQueryResultEvent(msg *message.Message) (QueryResultEvent, error) {
    var event QueryResultEvent
    if err := json.Unmarshal(msg.Payload, &event); err != nil {
        return event, fmt.Errorf("parsing query result event: %w", err)
    }
    return event, nil
}
```

### 4.4 Slog Logger Adapter

```go
// internal/pubsub/logger.go

package pubsub

import (
    "log/slog"

    "github.com/ThreeDotsLabs/watermill"
)

// SlogAdapter adapts slog.Logger to Watermill's LoggerAdapter interface.
type SlogAdapter struct {
    logger *slog.Logger
    fields watermill.LogFields
}

// NewSlogAdapter creates a new adapter.
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
    return &SlogAdapter{logger: logger}
}

func (s *SlogAdapter) Error(msg string, err error, fields watermill.LogFields) {
    s.logger.Error(msg, s.toAttrs(fields, err)...)
}

func (s *SlogAdapter) Info(msg string, fields watermill.LogFields) {
    s.logger.Info(msg, s.toAttrs(fields, nil)...)
}

func (s *SlogAdapter) Debug(msg string, fields watermill.LogFields) {
    s.logger.Debug(msg, s.toAttrs(fields, nil)...)
}

func (s *SlogAdapter) Trace(msg string, fields watermill.LogFields) {
    s.logger.Debug(msg, s.toAttrs(fields, nil)...) // slog has no trace
}

func (s *SlogAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
    merged := make(watermill.LogFields)
    for k, v := range s.fields {
        merged[k] = v
    }
    for k, v := range fields {
        merged[k] = v
    }
    return &SlogAdapter{logger: s.logger, fields: merged}
}

func (s *SlogAdapter) toAttrs(fields watermill.LogFields, err error) []any {
    attrs := make([]any, 0, len(s.fields)+len(fields)+2)
    for k, v := range s.fields {
        attrs = append(attrs, k, v)
    }
    for k, v := range fields {
        attrs = append(attrs, k, v)
    }
    if err != nil {
        attrs = append(attrs, "error", err)
    }
    return attrs
}
```

### 4.5 Handler Changes

#### DistributedWrite Handler (Publishing)

```go
// features/osquery/handlers.go

type Handlers struct {
    repo       hostRepository
    orgService enrollmentOrgLookup
    publisher  message.Publisher  // NEW: Watermill publisher
}

func NewHandlers(
    repo hostRepository,
    orgService enrollmentOrgLookup,
    publisher message.Publisher,  // NEW
) *Handlers {
    return &Handlers{
        repo:       repo,
        orgService: orgService,
        publisher:  publisher,
    }
}

func (h *Handlers) DistributedWrite(w http.ResponseWriter, r *http.Request) {
    // ... existing validation and parsing ...

    for queryIDStr, statusCode := range req.Statuses {
        queryID, err := uuid.Parse(queryIDStr)
        // ... existing error handling ...

        status := "completed"
        var errorText *string
        if statusCode != 0 {
            status = "failed"
            s := fmt.Sprintf("osquery status %d", statusCode)
            errorText = &s
        }

        // ... existing result marshaling ...

        if err := h.repo.SaveQueryResults(r.Context(), host.ID, queryID, status, resJSON, errorText); err != nil {
            slog.Error("failed to save query results", "error", err)
            continue  // Don't fail the whole request
        }

        // NEW: Publish event for SSE subscribers
        if h.publisher != nil {
            event := pubsub.QueryResultEvent{
                HostID:     host.ID,
                QueryID:    queryID,
                Status:     status,
                OccurredAt: time.Now(),
                Error:      errorText,
            }
            topic := pubsub.TopicQueryResults(host.ID)
            if err := h.publisher.Publish(topic, event.ToMessage()); err != nil {
                // Log but don't fail - results are saved, pub is best-effort
                slog.Error("failed to publish query result event",
                    "error", err,
                    "host_id", host.ID,
                    "query_id", queryID,
                )
            }
        }
    }

    h.jsonResponse(w, DistributedWriteResponse{})
}
```

#### HostResultsSSE Handler (Subscribing)

```go
// features/osquery/handlers.go

type Handlers struct {
    repo         hostRepository
    orgService   enrollmentOrgLookup
    publisher    message.Publisher
    pubsub       *pubsub.PubSub  // NEW: For creating subscribers
}

func (h *Handlers) HostResultsSSE(w http.ResponseWriter, r *http.Request) {
    // ... existing validation (host ID, org check) ...

    ctx := r.Context()
    sse := datastar.NewSSE(w, r)

    // 1. Send initial state immediately
    results, err := h.repo.GetRecentResults(ctx, hostID)
    if err != nil {
        _ = sse.ConsoleError(err)
        return
    }
    if err := sse.PatchElementTempl(pages.HostResultsTable(hostIDStr, results)); err != nil {
        return
    }

    // 2. If pub/sub is not available, fall back to polling
    if h.pubsub == nil {
        h.pollResultsLegacy(ctx, sse, hostID, hostIDStr, results)
        return
    }

    // 3. Subscribe to query result events for this host
    subscriber, err := h.pubsub.NewSubscriber(ctx)
    if err != nil {
        slog.Error("failed to create subscriber, falling back to polling", "error", err)
        h.pollResultsLegacy(ctx, sse, hostID, hostIDStr, results)
        return
    }
    defer subscriber.Close()

    topic := pubsub.TopicQueryResults(hostID)
    messages, err := subscriber.Subscribe(ctx, topic)
    if err != nil {
        slog.Error("failed to subscribe, falling back to polling", "error", err)
        h.pollResultsLegacy(ctx, sse, hostID, hostIDStr, results)
        return
    }

    // 4. Stream updates as events arrive
    for {
        select {
        case <-ctx.Done():
            return
            
        case msg := <-messages:
            if msg == nil {
                // Channel closed, subscriber shutting down
                return
            }
            
            event, err := pubsub.ParseQueryResultEvent(msg)
            if err != nil {
                slog.Error("failed to parse event", "error", err)
                msg.Nack()
                continue
            }
            
            // Fetch fresh results to display
            // (Alternative: Use event data directly for single-row update)
            results, err := h.repo.GetRecentResults(ctx, hostID)
            if err != nil {
                slog.Error("failed to get results after event", "error", err)
                msg.Nack()
                continue
            }
            
            if err := sse.PatchElementTempl(pages.HostResultsTable(hostIDStr, results)); err != nil {
                msg.Nack()
                return
            }
            
            msg.Ack()
        }
    }
}

// pollResultsLegacy is the fallback polling implementation.
func (h *Handlers) pollResultsLegacy(
    ctx context.Context,
    sse *datastar.ServerSentEventGenerator,
    hostID uuid.UUID,
    hostIDStr string,
    initialResults []services.QueryResult,
) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    last, _ := json.Marshal(initialResults)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            results, err := h.repo.GetRecentResults(ctx, hostID)
            if err != nil {
                _ = sse.ConsoleError(err)
                return
            }

            b, _ := json.Marshal(results)
            if !bytes.Equal(b, last) {
                last = b
                if err := sse.PatchElementTempl(pages.HostResultsTable(hostIDStr, results)); err != nil {
                    return
                }
            }
        }
    }
}
```

---

## 5. Database Schema

### 5.1 Watermill Tables

Watermill's `DefaultPostgreSQLSchema` creates tables with this structure:

```sql
-- Messages table (one per topic, or shared with topic column)
CREATE TABLE IF NOT EXISTS watermill_messages (
    "offset" BIGSERIAL PRIMARY KEY,
    "uuid" VARCHAR(36) NOT NULL,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "payload" JSONB NOT NULL,
    "metadata" JSONB NOT NULL,
    "topic" VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_watermill_messages_topic_offset 
    ON watermill_messages (topic, "offset");

-- Offsets table (tracks consumer group positions)
CREATE TABLE IF NOT EXISTS watermill_offsets (
    consumer_group VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    offset_acked BIGINT NOT NULL,
    last_processed_transaction_id XID8,
    PRIMARY KEY (consumer_group, topic)
);
```

### 5.2 Migration Strategy

**Option A: Auto-initialize (Development)**
- Set `AutoInitializeSchema: true` in config
- Watermill creates tables on first publish/subscribe
- Simple, but less control

**Option B: Explicit Migration (Production)**
- Create migration file: `migrations/sql/YYYYMMDDHHMMSS_watermill_pubsub.up.sql`
- Set `AutoInitializeSchema: false` in config
- Full control, audit trail

**Recommendation:** Option B for production safety. The migration should be idempotent.

### 5.3 Message Retention

By default, Watermill keeps all messages. For this use case (ephemeral notifications), consider:

1. **Periodic cleanup job** - Delete messages older than N hours
2. **Use Queue schema** - `DeleteOnAck: true` removes messages after processing
3. **PostgreSQL partitioning** - Partition by date for easy cleanup

For MVP, we'll use default behavior and add cleanup later if needed.

---

## 6. API Changes

### 6.1 No External API Changes

The feature is entirely internal. No changes to:
- HTTP routes
- Request/response formats
- osquery TLS protocol

### 6.2 Internal Interface Changes

```go
// features/osquery/handlers.go

// Before
func NewHandlers(repo hostRepository, orgService enrollmentOrgLookup) *Handlers

// After
func NewHandlers(
    repo hostRepository,
    orgService enrollmentOrgLookup,
    publisher message.Publisher,  // Can be nil for graceful degradation
    pubsub *pubsub.PubSub,        // Can be nil for graceful degradation
) *Handlers
```

### 6.3 Graceful Degradation

If pub/sub initialization fails:
- Handlers continue to work
- SSE falls back to polling
- No user-visible errors

---

## 7. Implementation Phases

### Phase 1: Infrastructure Foundation

**Goal:** Set up Watermill with PostgreSQL, verify basic pub/sub works.

**Tasks:**
1. Add dependencies to `go.mod`
2. Create `internal/pubsub/` package with core types
3. Implement slog adapter for Watermill logging
4. Create database migration for Watermill tables
5. Add PubSub initialization to `cmd/web/web.go`
6. Write integration test proving publish/subscribe works

**Acceptance Criteria:**
- `go build` succeeds with new dependencies
- Migration runs successfully
- Test demonstrates message round-trip

### Phase 2: Publish Query Results

**Goal:** Publish events when distributed query results are saved.

**Tasks:**
1. Define `QueryResultEvent` type
2. Add publisher to `Handlers` struct
3. Modify `DistributedWrite` to publish after `SaveQueryResults`
4. Update route setup to inject publisher
5. Add unit tests with mock publisher

**Acceptance Criteria:**
- `DistributedWrite` publishes event on success
- Publish failures are logged but don't fail the request
- Unit tests pass with mock publisher

### Phase 3: Subscribe and Stream

**Goal:** SSE handler subscribes to events and pushes updates.

**Tasks:**
1. Add `pubsub` field to `Handlers` struct
2. Refactor `HostResultsSSE` to use subscription
3. Implement fallback to polling if subscription fails
4. Extract polling logic to separate method
5. Add integration tests for SSE streaming

**Acceptance Criteria:**
- SSE receives updates within 100ms of publish
- Graceful fallback to polling on errors
- Multiple browsers can subscribe to same host

### Phase 4: Wiring and Integration

**Goal:** Wire everything together, end-to-end testing.

**Tasks:**
1. Update `router.SetupRoutes` to pass pub/sub
2. Update `osqueryFeature.SetupProtectedRoutes` signature
3. Add configuration options for pub/sub
4. Create end-to-end test scenario
5. Document configuration in README

**Acceptance Criteria:**
- Full flow works: submit query → agent returns → browser updates
- Configuration documented
- No regressions in existing tests

### Phase 5: Polish and Monitoring

**Goal:** Production readiness.

**Tasks:**
1. Add metrics for publish/subscribe operations
2. Implement message cleanup strategy
3. Add health check for pub/sub system
4. Load testing with many concurrent subscribers
5. Update operational documentation

**Acceptance Criteria:**
- Metrics visible in monitoring
- System handles 100+ concurrent subscribers
- Cleanup prevents unbounded table growth

---

## 8. Testing Strategy

### 8.1 Unit Tests

**Publisher tests:**
```go
func TestDistributedWrite_PublishesEvent(t *testing.T) {
    mockPublisher := &MockPublisher{}
    handlers := NewHandlers(mockRepo, mockOrgService, mockPublisher, nil)
    
    // ... setup request ...
    
    handlers.DistributedWrite(w, r)
    
    assert.Equal(t, 1, mockPublisher.PublishCount)
    assert.Equal(t, "query_results:host-uuid", mockPublisher.LastTopic)
}

func TestDistributedWrite_PublishFailure_DoesNotFailRequest(t *testing.T) {
    mockPublisher := &MockPublisher{Error: errors.New("publish failed")}
    handlers := NewHandlers(mockRepo, mockOrgService, mockPublisher, nil)
    
    // ... setup request ...
    
    handlers.DistributedWrite(w, r)
    
    assert.Equal(t, http.StatusOK, w.Code)  // Request succeeds
}
```

**Subscriber tests:**
```go
func TestHostResultsSSE_ReceivesEvents(t *testing.T) {
    // Integration test with real Watermill
}

func TestHostResultsSSE_FallbackToPolling(t *testing.T) {
    handlers := NewHandlers(mockRepo, mockOrgService, nil, nil)  // No pubsub
    
    // ... verify polling behavior ...
}
```

### 8.2 Integration Tests

Use `testdb` package with real PostgreSQL:

```go
func TestPubSub_Integration(t *testing.T) {
    pool := testdb.NewPool(t)
    
    ps, err := pubsub.New(context.Background(), pool, nil)
    require.NoError(t, err)
    defer ps.Close()
    
    // Publish
    event := pubsub.QueryResultEvent{...}
    err = ps.Publisher().Publish("test-topic", event.ToMessage())
    require.NoError(t, err)
    
    // Subscribe
    sub, err := ps.NewSubscriber(context.Background())
    require.NoError(t, err)
    defer sub.Close()
    
    messages, err := sub.Subscribe(context.Background(), "test-topic")
    require.NoError(t, err)
    
    // Receive
    select {
    case msg := <-messages:
        received, _ := pubsub.ParseQueryResultEvent(msg)
        assert.Equal(t, event.QueryID, received.QueryID)
        msg.Ack()
    case <-time.After(5 * time.Second):
        t.Fatal("timeout waiting for message")
    }
}
```

### 8.3 End-to-End Tests

Manual or automated browser testing:

1. Open host details page in browser A
2. Open host details page in browser B
3. Submit query via API/CLI simulating osquery agent
4. Verify both browsers update simultaneously

### 8.4 Load Tests

```go
func BenchmarkManySubscribers(b *testing.B) {
    // Create 100 subscribers to same topic
    // Publish 1000 messages
    // Verify all subscribers receive all messages
    // Measure latency distribution
}
```

---

## 9. Rollout Plan

### 9.1 Development Environment

1. Merge feature branch
2. Run `go tool task migrate` (creates Watermill tables)
3. Restart dev server
4. Test manually with osquery agent

### 9.2 Staging Environment

1. Deploy with feature flag disabled
2. Run migrations
3. Enable feature flag
4. Monitor for errors
5. Load test with synthetic traffic

### 9.3 Production Environment

1. Deploy with feature flag disabled
2. Run migrations during maintenance window
3. Enable feature flag for subset of users
4. Monitor metrics and error rates
5. Gradual rollout to all users

### 9.4 Rollback Plan

If issues arise:
1. Disable feature flag (reverts to polling)
2. No database rollback needed (Watermill tables are additive)
3. Investigate and fix
4. Re-enable after fix deployed

---

## 10. Alternatives Considered

### 10.1 Redis Pub/Sub

**Pros:**
- Very fast
- True push (no polling)
- Well-understood

**Cons:**
- Requires new infrastructure
- No persistence
- No transactional publish with Postgres

**Verdict:** Rejected - adds operational complexity

### 10.2 PostgreSQL LISTEN/NOTIFY

**Pros:**
- Native PostgreSQL
- No new dependencies
- True push

**Cons:**
- Limited payload size (8KB)
- No persistence/replay
- Connection-based (each listener needs connection)
- pgx pool doesn't support LISTEN well

**Verdict:** Rejected - connection management complexity

### 10.3 River Jobs (existing infrastructure)

**Pros:**
- Already in codebase
- Uses same pgx pool
- Persistent

**Cons:**
- Job queue, not pub/sub
- No fan-out pattern
- Would require polling job completion

**Verdict:** Rejected - wrong abstraction for real-time streaming

### 10.4 Server-Sent Events with Polling (current)

**Pros:**
- Simple
- Already working
- No new dependencies

**Cons:**
- Latency
- Database load
- Doesn't scale

**Verdict:** Keep as fallback, but improve primary path

### 10.5 WebSocket with Custom Protocol

**Pros:**
- Bidirectional
- Lower overhead than SSE

**Cons:**
- More complex than SSE
- Need custom protocol
- Datastar already uses SSE

**Verdict:** Rejected - overengineered for this use case

---

## 11. Open Questions

### 11.1 Resolved

| Question | Decision | Rationale |
|----------|----------|-----------|
| Topic granularity? | Per-host | Matches UI subscription pattern |
| Publish in transaction? | No | Best-effort is fine, results are already saved |
| Fallback behavior? | Yes, to polling | Graceful degradation is important |

### 11.2 To Be Decided

| Question | Options | Recommendation |
|----------|---------|----------------|
| Message retention | Keep all / Delete on ack / TTL cleanup | Start with keep all, add cleanup job later |
| Consumer groups | Per-connection / Shared | Per-connection (simpler) |
| Metrics format | Prometheus / StatsD / slog | Prometheus (matches industry standard) |
| Feature flag | Config / DB / None | Config (`PUBSUB_ENABLED=true`) |

### 11.3 Future Considerations

- **Multi-region:** Would need distributed pub/sub (Redis, NATS)
- **Very high fan-out:** Consider in-memory fan-out layer
- **Message ordering:** Watermill guarantees per-topic ordering
- **Exactly-once delivery:** Not needed for UI updates (idempotent)

---

## 12. References

### Documentation

- [Watermill Documentation](https://watermill.io/)
- [Watermill SQL Pub/Sub](https://watermill.io/pubsubs/sql/)
- [watermill-sql GitHub](https://github.com/ThreeDotsLabs/watermill-sql)
- [Datastar SSE](https://data-star.dev/)

### Code References

- `features/osquery/handlers.go` - Current implementation
- `features/osquery/services/host_repository.go` - Data layer
- `background/river.go` - Existing background job pattern
- `cmd/web/web.go` - Server initialization

### Related PRs/Issues

- TBD (will link after implementation)

---

## Appendix A: Watermill pgx Adapter Usage

```go
import (
    "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
    "github.com/jackc/pgx/v5/pgxpool"
)

// Create publisher from pgx pool
publisher, err := sql.NewPublisher(
    sql.BeginnerFromPgxPool(pool),
    sql.PublisherConfig{
        SchemaAdapter:        sql.DefaultPostgreSQLSchema{},
        AutoInitializeSchema: true,
    },
    logger,
)

// Create subscriber from pgx pool
subscriber, err := sql.NewSubscriber(
    sql.BeginnerFromPgxPool(pool),
    sql.SubscriberConfig{
        SchemaAdapter:    sql.DefaultPostgreSQLSchema{},
        OffsetsAdapter:   sql.DefaultPostgreSQLOffsetsAdapter{},
        PollInterval:     100 * time.Millisecond,
        InitializeSchema: false,
    },
    logger,
)
```

---

## Appendix B: Message Flow Sequence Diagram

```
sequenceDiagram
    participant B as Browser
    participant H as Handler
    participant R as Repository
    participant P as Publisher
    participant DB as PostgreSQL
    participant S as Subscriber
    participant SSE as SSE Handler

    Note over B,SSE: Initial Connection
    B->>SSE: GET /hosts/:id/results (SSE)
    SSE->>R: GetRecentResults()
    R->>DB: SELECT
    DB-->>R: results
    R-->>SSE: results
    SSE-->>B: SSE: initial data
    SSE->>S: Subscribe(topic)
    S->>DB: SELECT (poll for messages)
    
    Note over B,SSE: osquery Returns Results
    H->>R: SaveQueryResults()
    R->>DB: UPDATE distributed_query_targets
    DB-->>R: ok
    R-->>H: ok
    H->>P: Publish(event)
    P->>DB: INSERT watermill_messages
    DB-->>P: ok
    P-->>H: ok
    H-->>Agent: 200 OK
    
    Note over B,SSE: SSE Receives Event
    S->>DB: SELECT (poll for messages)
    DB-->>S: new message
    S-->>SSE: message channel
    SSE->>R: GetRecentResults()
    R->>DB: SELECT
    DB-->>R: updated results
    R-->>SSE: results
    SSE-->>B: SSE: updated data
    SSE->>S: Ack(message)
    S->>DB: UPDATE offset
```

---

*End of Proposal*
