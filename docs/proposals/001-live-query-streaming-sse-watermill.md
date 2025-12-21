# Proposal 001: Live Query Results Streaming via SSE with Watermill Pub/Sub

**Status:** In Progress (Pivoting to Campaign-Based Design)  
**Author:** AI Assistant  
**Created:** 2025-12-20  
**Updated:** 2025-12-20 (Major revision: Pivot from host-based to campaign-based topics)  
**Related Issues:** See beads tracking (queryops-xuz epic)

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
│                       CAMPAIGN-BASED FLOW                               │
└─────────────────────────────────────────────────────────────────────────┘

  Browser                    Web Server                    Database
     │                           │                            │
     │  POST /api/v1/queries/run │                            │
     │  { query, host_ids? }     │                            │
     │ ─────────────────────────►│                            │
     │                           │  INSERT campaign           │
     │                           │  INSERT campaign_targets   │
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│◄───────────────────────────│
     │  { campaign_id, count }   │                            │
     │                           │                            │
     │  GET /campaigns/:id/results                            │
     │ ─────────────────────────►│                            │
     │        (SSE open)         │                            │
     │                           │  SELECT campaign + results │
     │◄──────────────────────────│◄───────────────────────────│
     │    Initial state          │                            │
     │                           │                            │
     │                           │  SUBSCRIBE campaign:{id}   │
     │                           │ ──────────────────────────►│
     │                           │        (waiting...)        │


  osquery Agent              Web Server                    Database
     │                           │                            │
     │  POST /distributed_read   │                            │
     │ ─────────────────────────►│                            │
     │                           │  SELECT pending queries    │
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│◄───────────────────────────│
     │   { queries: {...} }      │  (linked to campaign)      │
     │                           │                            │
     │   (executes query)        │                            │
     │                           │                            │
     │  POST /distributed_write  │                            │
     │ ─────────────────────────►│                            │
     │                           │  UPDATE campaign_targets   │
     │                           │ ──────────────────────────►│
     │                           │                            │
     │                           │  PUBLISH campaign:{id}     │
     │                           │ ──────────────────────────►│
     │◄──────────────────────────│                            │
     │        200 OK             │                            │


  Web Server (SSE Handler)                               Database
     │                                                      │
     │        (receives CampaignResultEvent)                │
     │◄─────────────────────────────────────────────────────│
     │                                                      │
     │  SELECT campaign + results                           │
     │ ────────────────────────────────────────────────────►│
     │◄─────────────────────────────────────────────────────│
     │                                                      │
     │  PUSH to browser via SSE                             │
     │  (renders updated campaign progress)                 │
     │                                                      │


  Browser                    Web Server
     │                           │
     │◄──────────────────────────│
     │   SSE: Host X completed   │
     │   SSE: Host Y completed   │
     │   SSE: Campaign done!     │
     │   (stream closes)         │
```

### Topic Design

**Topic naming:** `campaign:{campaign_id}`

Each distributed query execution creates a **campaign** — a first-class entity that tracks the query execution across one or more hosts. When a user submits a query, they receive a `campaign_id` and can subscribe to that campaign's results via a dedicated SSE endpoint.

**Key Concepts:**
- **Campaign:** A unique execution of a query, potentially targeting multiple hosts
- **Campaign ID:** UUID returned when submitting a query, used to subscribe to results
- **Topic:** `campaign:{campaign_id}` — each campaign has its own pub/sub topic

**Rationale:**
- **Dedicated query execution experience:** Submit query → get campaign ID → watch just that query's results
- **Filtering:** Hosts may return results from many queries; campaign-based topics filter to just "your" query
- **Multi-host queries:** A campaign can target multiple hosts; results from all hosts aggregate into one stream
- **Audit/history:** Campaign IDs provide a first-class entity for tracking execution history
- **Clean separation:** Host details page can still show recent results, but live query execution gets its own UX

---

## 4. Technical Design

### 4.1 Package Structure

```
internal/
└── pubsub/
    ├── pubsub.go        # PubSub struct, New(), Close()
    ├── publisher.go     # Publisher wrapper and helpers
    ├── subscriber.go    # Subscriber wrapper and helpers
    ├── events.go        # Event type definitions (campaign events)
    ├── schema.go        # Custom PostgreSQL schema adapter (shared tables)
    └── logger.go        # Watermill logger adapter for slog

features/
└── osquery/
    └── services/
        └── campaign_repository.go  # Campaign CRUD operations
```

### 4.1.1 Campaign Entity

A **Campaign** represents a single execution of a distributed query:

```go
// features/osquery/services/campaign.go

type Campaign struct {
    ID          uuid.UUID  `json:"id"`
    OrgID       uuid.UUID  `json:"org_id"`
    Query       string     `json:"query"`
    CreatedAt   time.Time  `json:"created_at"`
    CreatedBy   uuid.UUID  `json:"created_by"`      // User who initiated
    Status      string     `json:"status"`          // pending, running, completed, failed
    TargetCount int        `json:"target_count"`    // Number of hosts targeted
    ResultCount int        `json:"result_count"`    // Number of results received
}

type CampaignTarget struct {
    CampaignID  uuid.UUID  `json:"campaign_id"`
    HostID      uuid.UUID  `json:"host_id"`
    Status      string     `json:"status"`          // pending, sent, completed, failed
    SentAt      *time.Time `json:"sent_at,omitempty"`
    ResultAt    *time.Time `json:"result_at,omitempty"`
    Result      *string    `json:"result,omitempty"`
    Error       *string    `json:"error,omitempty"`
}
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

// TopicCampaign returns the topic name for a campaign's results.
func TopicCampaign(campaignID uuid.UUID) string {
    return fmt.Sprintf("campaign:%s", campaignID.String())
}

// CampaignResultEvent is published when a host returns results for a campaign.
type CampaignResultEvent struct {
    // CampaignID is the campaign this result belongs to.
    CampaignID uuid.UUID `json:"campaign_id"`
    
    // HostID is the host that executed the query.
    HostID uuid.UUID `json:"host_id"`
    
    // HostIdentifier is the human-readable host identifier.
    HostIdentifier string `json:"host_identifier"`
    
    // Status is the result status: "completed" or "failed".
    Status string `json:"status"`
    
    // OccurredAt is when the result was saved.
    OccurredAt time.Time `json:"occurred_at"`
    
    // RowCount is the number of result rows (for completed status).
    RowCount int `json:"row_count,omitempty"`
    
    // Error is set if status is "failed".
    Error *string `json:"error,omitempty"`
}

// ToMessage converts the event to a Watermill message.
func (e CampaignResultEvent) ToMessage() *message.Message {
    payload, _ := json.Marshal(e)
    msg := message.NewMessage(uuid.New().String(), payload)
    msg.Metadata.Set("event_type", "campaign_result")
    msg.Metadata.Set("campaign_id", e.CampaignID.String())
    msg.Metadata.Set("host_id", e.HostID.String())
    return msg
}

// ParseCampaignResultEvent parses a Watermill message into a CampaignResultEvent.
func ParseCampaignResultEvent(msg *message.Message) (CampaignResultEvent, error) {
    var event CampaignResultEvent
    if err := json.Unmarshal(msg.Payload, &event); err != nil {
        return event, fmt.Errorf("parsing campaign result event: %w", err)
    }
    return event, nil
}

// CampaignStatusEvent is published when the overall campaign status changes.
type CampaignStatusEvent struct {
    CampaignID  uuid.UUID `json:"campaign_id"`
    Status      string    `json:"status"`       // running, completed, failed
    OccurredAt  time.Time `json:"occurred_at"`
    ResultCount int       `json:"result_count"` // Total results received so far
    TargetCount int       `json:"target_count"` // Total hosts targeted
}

// ToMessage converts the event to a Watermill message.
func (e CampaignStatusEvent) ToMessage() *message.Message {
    payload, _ := json.Marshal(e)
    msg := message.NewMessage(uuid.New().String(), payload)
    msg.Metadata.Set("event_type", "campaign_status")
    msg.Metadata.Set("campaign_id", e.CampaignID.String())
    return msg
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

#### RunQuery Handler (Create Campaign)

```go
// features/osquery/handlers.go

type RunQueryRequest struct {
    Query   string      `json:"query"`
    HostIDs []uuid.UUID `json:"host_ids,omitempty"` // Optional: specific hosts
}

type RunQueryResponse struct {
    CampaignID  uuid.UUID `json:"campaign_id"`
    TargetCount int       `json:"target_count"`
}

func (h *Handlers) RunQuery(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    org := organization.FromContext(ctx)
    user := auth.UserFromContext(ctx)
    
    var req RunQueryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }
    
    // Create campaign
    campaign, err := h.repo.CreateCampaign(ctx, services.CreateCampaignParams{
        OrgID:     org.ID,
        Query:     req.Query,
        CreatedBy: user.ID,
        HostIDs:   req.HostIDs, // nil = all hosts in org
    })
    if err != nil {
        slog.Error("failed to create campaign", "error", err)
        http.Error(w, "failed to create campaign", http.StatusInternalServerError)
        return
    }
    
    h.jsonResponse(w, RunQueryResponse{
        CampaignID:  campaign.ID,
        TargetCount: campaign.TargetCount,
    })
}
```

#### DistributedWrite Handler (Publishing to Campaign Topic)

```go
// features/osquery/handlers.go

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

        // Save results and get campaign ID
        campaignID, err := h.repo.SaveQueryResults(r.Context(), host.ID, queryID, status, resJSON, errorText)
        if err != nil {
            slog.Error("failed to save query results", "error", err)
            continue
        }

        // Publish event to campaign topic for SSE subscribers
        if h.publisher != nil && campaignID != nil {
            event := pubsub.CampaignResultEvent{
                CampaignID:     *campaignID,
                HostID:         host.ID,
                HostIdentifier: host.HostIdentifier,
                Status:         status,
                OccurredAt:     time.Now(),
                RowCount:       len(req.Queries[queryIDStr]),
                Error:          errorText,
            }
            topic := pubsub.TopicCampaign(*campaignID)
            if err := h.publisher.Publish(topic, event.ToMessage()); err != nil {
                slog.Error("failed to publish campaign result event",
                    "error", err,
                    "campaign_id", campaignID,
                    "host_id", host.ID,
                )
            }
        }
    }

    h.jsonResponse(w, DistributedWriteResponse{})
}
```

#### CampaignResultsSSE Handler (New Endpoint)

```go
// features/osquery/handlers.go

// CampaignResultsSSE streams results for a specific campaign via SSE.
// GET /campaigns/:id/results
func (h *Handlers) CampaignResultsSSE(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    org := organization.FromContext(ctx)
    
    campaignIDStr := chi.URLParam(r, "id")
    campaignID, err := uuid.Parse(campaignIDStr)
    if err != nil {
        http.Error(w, "invalid campaign id", http.StatusBadRequest)
        return
    }
    
    // Verify campaign belongs to org
    campaign, err := h.repo.GetCampaign(ctx, campaignID)
    if err != nil {
        http.Error(w, "campaign not found", http.StatusNotFound)
        return
    }
    if campaign.OrgID != org.ID {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    
    sse := datastar.NewSSE(w, r)
    
    // 1. Send initial state (campaign info + any existing results)
    results, err := h.repo.GetCampaignResults(ctx, campaignID)
    if err != nil {
        _ = sse.ConsoleError(err)
        return
    }
    if err := sse.PatchElementTempl(pages.CampaignResults(campaign, results)); err != nil {
        return
    }
    
    // 2. If campaign already completed, no need to subscribe
    if campaign.Status == "completed" || campaign.Status == "failed" {
        return
    }
    
    // 3. If pub/sub not available, fall back to polling
    if h.pubsub == nil {
        h.pollCampaignResultsLegacy(ctx, sse, campaign, results)
        return
    }
    
    // 4. Subscribe to campaign topic
    subscriber, err := h.pubsub.NewSubscriber(ctx)
    if err != nil {
        slog.Error("failed to create subscriber, falling back to polling", "error", err)
        h.pollCampaignResultsLegacy(ctx, sse, campaign, results)
        return
    }
    defer subscriber.Close()
    
    topic := pubsub.TopicCampaign(campaignID)
    messages, err := subscriber.Subscribe(ctx, topic)
    if err != nil {
        slog.Error("failed to subscribe, falling back to polling", "error", err)
        h.pollCampaignResultsLegacy(ctx, sse, campaign, results)
        return
    }
    
    // 5. Stream updates as results arrive
    for {
        select {
        case <-ctx.Done():
            return
            
        case msg := <-messages:
            if msg == nil {
                return
            }
            
            event, err := pubsub.ParseCampaignResultEvent(msg)
            if err != nil {
                slog.Error("failed to parse event", "error", err)
                msg.Nack()
                continue
            }
            
            // Re-fetch campaign and results
            campaign, err = h.repo.GetCampaign(ctx, campaignID)
            if err != nil {
                msg.Nack()
                continue
            }
            
            results, err = h.repo.GetCampaignResults(ctx, campaignID)
            if err != nil {
                msg.Nack()
                continue
            }
            
            if err := sse.PatchElementTempl(pages.CampaignResults(campaign, results)); err != nil {
                msg.Nack()
                return
            }
            
            msg.Ack()
            
            // If campaign completed, close stream
            if campaign.Status == "completed" || campaign.Status == "failed" {
                return
            }
        }
    }
}

// pollCampaignResultsLegacy is the fallback polling implementation.
func (h *Handlers) pollCampaignResultsLegacy(
    ctx context.Context,
    sse *datastar.ServerSentEventGenerator,
    campaign *services.Campaign,
    initialResults []services.CampaignResult,
) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    last, _ := json.Marshal(initialResults)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            campaign, err := h.repo.GetCampaign(ctx, campaign.ID)
            if err != nil {
                _ = sse.ConsoleError(err)
                return
            }
            
            results, err := h.repo.GetCampaignResults(ctx, campaign.ID)
            if err != nil {
                _ = sse.ConsoleError(err)
                return
            }

            b, _ := json.Marshal(results)
            if !bytes.Equal(b, last) {
                last = b
                if err := sse.PatchElementTempl(pages.CampaignResults(campaign, results)); err != nil {
                    return
                }
            }
            
            // Stop polling if campaign completed
            if campaign.Status == "completed" || campaign.Status == "failed" {
                return
            }
        }
    }
}
```

---

## 5. Database Schema

### 5.1 Campaign Tables (New)

```sql
-- Campaigns table tracks distributed query executions
CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
    target_count INT NOT NULL DEFAULT 0,
    result_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_campaigns_org_id ON campaigns(org_id);
CREATE INDEX idx_campaigns_created_at ON campaigns(created_at DESC);
CREATE INDEX idx_campaigns_status ON campaigns(status) WHERE status IN ('pending', 'running');

-- Campaign targets tracks which hosts are targeted and their results
CREATE TABLE IF NOT EXISTS campaign_targets (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, sent, completed, failed
    sent_at TIMESTAMPTZ,
    result_at TIMESTAMPTZ,
    result JSONB,
    error TEXT,
    PRIMARY KEY (campaign_id, host_id)
);

CREATE INDEX idx_campaign_targets_host_id ON campaign_targets(host_id);
CREATE INDEX idx_campaign_targets_pending ON campaign_targets(campaign_id) WHERE status = 'pending';
```

### 5.2 Watermill Tables (Shared Schema)

**Important:** We use a custom shared-table schema because Watermill's default PostgreSQL schema creates **per-topic tables**. With dynamic topics like `campaign:{uuid}`, that would create an unbounded number of tables.

```sql
-- Shared messages table for all topics
CREATE TABLE IF NOT EXISTS watermill_messages (
    "offset" BIGSERIAL,
    "uuid" VARCHAR(36) NOT NULL,
    "topic" VARCHAR(255) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "payload" JSONB NOT NULL,
    "metadata" JSONB NOT NULL,
    "transaction_id" XID8 NOT NULL DEFAULT pg_current_xact_id(),
    PRIMARY KEY ("topic", "offset")
);

CREATE INDEX idx_watermill_messages_created_at ON watermill_messages(created_at);

-- Shared offsets table for all consumer groups
CREATE TABLE IF NOT EXISTS watermill_offsets (
    consumer_group VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    offset_acked BIGINT NOT NULL DEFAULT 0,
    last_processed_transaction_id XID8,
    PRIMARY KEY (consumer_group, topic)
);
```

### 5.3 Migration Strategy

We use **explicit migrations** (already implemented):
- `migrations/sql/20251221022000_watermill_pubsub.up.sql` - creates shared Watermill tables
- Campaign tables will need a new migration

### 5.4 Message Retention

With campaign-based topics:
- Messages are only relevant while the campaign is active
- Once a campaign completes, messages can be cleaned up
- **Cleanup strategy:** Background job deletes messages for completed campaigns older than N hours

```sql
-- Cleanup completed campaign messages (run periodically)
DELETE FROM watermill_messages 
WHERE topic LIKE 'campaign:%' 
AND created_at < NOW() - INTERVAL '24 hours';
```

---

## 6. API Changes

### 6.1 New HTTP Endpoints

#### Run Query (Create Campaign)

```
POST /api/v1/queries/run
Authorization: Bearer <token>
Content-Type: application/json

{
    "query": "SELECT * FROM processes WHERE name = 'nginx';",
    "host_ids": ["uuid1", "uuid2"]  // Optional: omit for all hosts in org
}

Response 201:
{
    "campaign_id": "550e8400-e29b-41d4-a716-446655440000",
    "target_count": 15
}
```

#### Campaign Results SSE Stream

```
GET /api/v1/campaigns/{id}/results
Authorization: Bearer <token>
Accept: text/event-stream

Response: SSE stream with campaign progress updates
```

#### Get Campaign Status

```
GET /api/v1/campaigns/{id}
Authorization: Bearer <token>

Response 200:
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "query": "SELECT * FROM processes WHERE name = 'nginx';",
    "status": "running",
    "created_at": "2025-12-20T10:00:00Z",
    "target_count": 15,
    "result_count": 8
}
```

#### List Campaigns

```
GET /api/v1/campaigns
Authorization: Bearer <token>

Response 200:
{
    "campaigns": [
        {
            "id": "...",
            "query": "...",
            "status": "completed",
            "created_at": "...",
            "target_count": 15,
            "result_count": 15
        }
    ]
}
```

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

### 6.3 osquery Protocol (Unchanged)

The osquery TLS endpoints remain unchanged:
- `POST /distributed_read` - returns pending queries (now linked to campaigns)
- `POST /distributed_write` - receives results (now publishes to campaign topic)

The campaign system is transparent to osquery agents.

### 6.4 Graceful Degradation

If pub/sub initialization fails:
- Handlers continue to work
- SSE falls back to polling
- No user-visible errors

---

## 7. Implementation Phases

### Phase 1: Infrastructure Foundation ✅ (Completed)

**Goal:** Set up Watermill with PostgreSQL, verify basic pub/sub works.

**Status:** DONE - Watermill with shared-table schema implemented.

**Completed:**
- ✅ Added Watermill dependencies to `go.mod`
- ✅ Created `internal/pubsub/` package with core types
- ✅ Implemented slog adapter for Watermill logging
- ✅ Created database migration for shared Watermill tables
- ✅ Added PubSub initialization to `cmd/web/web.go`
- ✅ Integration tests proving publish/subscribe works

### Phase 2: Campaign Entity & Database

**Goal:** Create the campaign data model and persistence layer.

**Tasks:**
1. Create migration for `campaigns` and `campaign_targets` tables
2. Create `CampaignRepository` with CRUD operations
3. Update `distributed_queries` to link to campaigns
4. Add tests for campaign repository

**Acceptance Criteria:**
- Migration runs successfully
- Can create campaigns, add targets, update results
- Query results correctly link back to campaigns

### Phase 3: Campaign API Endpoints

**Goal:** Expose campaign operations via HTTP API.

**Tasks:**
1. Implement `POST /api/v1/queries/run` (create campaign)
2. Implement `GET /api/v1/campaigns/{id}` (get campaign)
3. Implement `GET /api/v1/campaigns` (list campaigns)
4. Add route registration and middleware
5. Add API tests

**Acceptance Criteria:**
- Can create campaigns via API
- Campaign status and results accessible
- Proper org-scoping and authorization

### Phase 4: Campaign Results SSE Streaming

**Goal:** Stream campaign results via pub/sub + SSE.

**Tasks:**
1. Update event types: `CampaignResultEvent`, `TopicCampaign()`
2. Implement `GET /api/v1/campaigns/{id}/results` SSE endpoint
3. Modify `DistributedWrite` to publish to campaign topic
4. Implement fallback polling for graceful degradation
5. Add integration tests for SSE streaming

**Acceptance Criteria:**
- SSE receives updates within 100ms of publish
- Graceful fallback to polling on errors
- Multiple browsers can subscribe to same campaign
- Stream closes when campaign completes

### Phase 5: UI Integration

**Goal:** Build the "Live Query Execution" user experience.

**Tasks:**
1. Create campaign execution page template
2. Create campaign results component (Datastar/SSE)
3. Update query submission UI to show campaign ID
4. Add campaign history/list view
5. Update host details page to link to campaigns

**Acceptance Criteria:**
- User can submit query and watch results stream in
- Progress indicator shows hosts responding
- Can view historical campaigns

### Phase 6: Polish and Monitoring

**Goal:** Production readiness.

**Tasks:**
1. Add metrics for publish/subscribe operations
2. Implement message cleanup strategy (completed campaigns)
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
| Topic granularity? | **Per-campaign** (changed from per-host) | Enables dedicated query execution UX, multi-host aggregation, and audit trail |
| Publish in transaction? | No | Best-effort is fine, results are already saved |
| Fallback behavior? | Yes, to polling | Graceful degradation is important |
| Shared vs per-topic tables? | Shared tables | Dynamic topics (`campaign:{uuid}`) would create unbounded tables |

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
    participant API as API Handler
    participant R as Repository
    participant P as Publisher
    participant DB as PostgreSQL
    participant S as Subscriber
    participant SSE as SSE Handler
    participant Agent as osquery Agent

    Note over B,Agent: 1. Create Campaign
    B->>API: POST /api/v1/queries/run
    API->>R: CreateCampaign(query, hosts)
    R->>DB: INSERT campaigns, campaign_targets
    DB-->>R: campaign
    R-->>API: campaign
    API-->>B: { campaign_id, target_count }
    
    Note over B,Agent: 2. Open SSE Stream
    B->>SSE: GET /campaigns/:id/results (SSE)
    SSE->>R: GetCampaign(), GetCampaignResults()
    R->>DB: SELECT
    DB-->>R: campaign, results
    R-->>SSE: data
    SSE-->>B: SSE: initial state (0/N hosts)
    SSE->>S: Subscribe(campaign:{id})
    S->>DB: SELECT (poll for messages)
    
    Note over B,Agent: 3. Agent Fetches Query
    Agent->>API: POST /distributed_read
    API->>R: GetPendingQueries(host)
    R->>DB: SELECT (campaign-linked queries)
    DB-->>R: queries
    R-->>API: queries
    API-->>Agent: { queries: {...} }
    
    Note over B,Agent: 4. Agent Returns Results
    Agent->>API: POST /distributed_write
    API->>R: SaveQueryResults()
    R->>DB: UPDATE campaign_targets
    DB-->>R: campaign_id
    R-->>API: ok
    API->>P: Publish(CampaignResultEvent)
    P->>DB: INSERT watermill_messages
    DB-->>P: ok
    P-->>API: ok
    API-->>Agent: 200 OK
    
    Note over B,Agent: 5. SSE Receives Event
    S->>DB: SELECT (poll for messages)
    DB-->>S: new message
    S-->>SSE: message channel
    SSE->>R: GetCampaign(), GetCampaignResults()
    R->>DB: SELECT
    DB-->>R: updated data
    R-->>SSE: data
    SSE-->>B: SSE: Host X completed (1/N)
    SSE->>S: Ack(message)
    S->>DB: UPDATE offset
    
    Note over B,Agent: 6. Campaign Completes
    SSE-->>B: SSE: Campaign completed (N/N)
    SSE->>SSE: Close stream
```

---

*End of Proposal*
