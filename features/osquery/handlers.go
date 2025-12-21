package osquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/starfederation/datastar-go/datastar"

	org "github.com/cavenine/queryops/features/organization"
	orgServices "github.com/cavenine/queryops/features/organization/services"
	"github.com/cavenine/queryops/features/osquery/pages"
	"github.com/cavenine/queryops/features/osquery/services"
	"github.com/cavenine/queryops/internal/pubsub"
)

type hostRepository interface {
	Enroll(ctx context.Context, hostIdentifier string, hostDetails json.RawMessage, organizationID uuid.UUID) (string, error)
	GetByNodeKey(ctx context.Context, nodeKey string) (*services.Host, error)
	UpdateLastConfig(ctx context.Context, nodeKey string) error
	UpdateLastLogger(ctx context.Context, nodeKey string) error
	UpdateLastDistributed(ctx context.Context, nodeKey string) error
	GetConfigForHost(ctx context.Context, nodeKey string) (json.RawMessage, error)
	SaveResultLogs(ctx context.Context, hostID uuid.UUID, name, action string, columns json.RawMessage, timestamp time.Time) error
	SaveStatusLogs(ctx context.Context, hostID uuid.UUID, line int, message string, severity int, filename string, createdAt time.Time) error
	GetPendingQueries(ctx context.Context, hostID uuid.UUID) (map[string]string, error)
	SaveQueryResults(ctx context.Context, hostID uuid.UUID, queryID uuid.UUID, status string, results json.RawMessage, errorText *string) error

	ListByOrganization(ctx context.Context, organizationID uuid.UUID) ([]*services.Host, error)
	GetByIDAndOrganization(ctx context.Context, id uuid.UUID, organizationID uuid.UUID) (*services.Host, error)
	GetRecentResults(ctx context.Context, hostID uuid.UUID) ([]services.QueryResult, error)
	QueueQuery(ctx context.Context, query string, hostIDs []uuid.UUID) (uuid.UUID, error)
}

type enrollmentOrgLookup interface {
	GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*orgServices.Organization, error)
}

type Handlers struct {
	repo       hostRepository
	orgService enrollmentOrgLookup
	publisher  message.Publisher
	pubsub     *pubsub.PubSub
}

// NewHandlers creates a new Handlers instance.
// publisher and pubsub can be nil for graceful degradation to polling.
func NewHandlers(
	repo hostRepository,
	orgService enrollmentOrgLookup,
	publisher message.Publisher,
	ps *pubsub.PubSub,
) *Handlers {
	return &Handlers{
		repo:       repo,
		orgService: orgService,
		publisher:  publisher,
		pubsub:     ps,
	}
}

func (h *Handlers) Enroll(w http.ResponseWriter, r *http.Request) {
	var req EnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	org, err := h.orgService.GetOrganizationByEnrollSecret(r.Context(), req.EnrollSecret)
	if err != nil {
		if errors.Is(err, orgServices.ErrOrganizationNotFound) {
			slog.Warn("invalid enrollment secret")
			h.jsonResponse(w, EnrollmentResponse{NodeInvalid: true})
			return
		}
		slog.Error("failed to look up organization by enroll secret", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if org == nil {
		slog.Error("organization lookup returned nil")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	nodeKey, err := h.repo.Enroll(r.Context(), req.HostIdentifier, req.HostDetails, org.ID)
	if err != nil {
		slog.Error("failed to enroll host", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, EnrollmentResponse{NodeKey: nodeKey})
}

func (h *Handlers) Config(w http.ResponseWriter, r *http.Request) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	host, err := h.repo.GetByNodeKey(r.Context(), req.NodeKey)
	if err != nil || host == nil {
		h.jsonResponse(w, ConfigResponse{NodeInvalid: true})
		return
	}

	if err := h.repo.UpdateLastConfig(r.Context(), req.NodeKey); err != nil {
		slog.Error("failed to update last config", "error", err)
	}

	configRaw, err := h.repo.GetConfigForHost(r.Context(), req.NodeKey)
	if err != nil {
		slog.Error("failed to get config for host", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var resp ConfigResponse
	if err := json.Unmarshal(configRaw, &resp); err != nil {
		slog.Error("failed to unmarshal config", "error", err, "raw", string(configRaw))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, resp)
}

func (h *Handlers) Logger(w http.ResponseWriter, r *http.Request) {
	var req LoggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	host, err := h.repo.GetByNodeKey(r.Context(), req.NodeKey)
	if err != nil || host == nil {
		h.jsonResponse(w, LoggerResponse{NodeInvalid: true})
		return
	}

	if err := h.repo.UpdateLastLogger(r.Context(), req.NodeKey); err != nil {
		slog.Error("failed to update last logger", "error", err)
	}

	slog.Info("received logs from host", "host_identifier", host.HostIdentifier, "log_type", req.LogType, "count", len(req.Data))

	for _, raw := range req.Data {
		if req.LogType == "result" {
			var log ResultLog
			if err := json.Unmarshal(raw, &log); err != nil {
				slog.Error("failed to unmarshal result log", "error", err)
				continue
			}
			ts := time.Unix(int64(log.UnixTime), 0)
			cols, err := json.Marshal(log.Columns)
			if err != nil {
				slog.Error("failed to marshal result log columns", "error", err)
				continue
			}
			if err := h.repo.SaveResultLogs(r.Context(), host.ID, log.Name, log.Action, json.RawMessage(cols), ts); err != nil {
				slog.Error("failed to save result log", "error", err)
			}
		} else if req.LogType == "status" {
			var log StatusLog
			if err := json.Unmarshal(raw, &log); err != nil {
				slog.Error("failed to unmarshal status log", "error", err)
				continue
			}
			ts := time.Unix(int64(log.UnixTime), 0)
			if err := h.repo.SaveStatusLogs(r.Context(), host.ID, log.Line, log.Message, log.Severity, log.Filename, ts); err != nil {
				slog.Error("failed to save status log", "error", err)
			}
		}
	}

	h.jsonResponse(w, LoggerResponse{})
}

func (h *Handlers) DistributedRead(w http.ResponseWriter, r *http.Request) {
	var req DistributedReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	host, err := h.repo.GetByNodeKey(r.Context(), req.NodeKey)
	if err != nil || host == nil {
		h.jsonResponse(w, DistributedReadResponse{NodeInvalid: true, Queries: map[string]string{}})
		return
	}

	if err := h.repo.UpdateLastDistributed(r.Context(), req.NodeKey); err != nil {
		slog.Error("failed to update last distributed", "error", err)
	}

	queries, err := h.repo.GetPendingQueries(r.Context(), host.ID)
	if err != nil {
		slog.Error("failed to get pending queries", "error", err)
		h.jsonResponse(w, DistributedReadResponse{Queries: map[string]string{}})
		return
	}

	h.jsonResponse(w, DistributedReadResponse{
		Queries: queries,
	})
}

func (h *Handlers) DistributedWrite(w http.ResponseWriter, r *http.Request) {
	var req DistributedWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	host, err := h.repo.GetByNodeKey(r.Context(), req.NodeKey)
	if err != nil || host == nil {
		h.jsonResponse(w, DistributedWriteResponse{NodeInvalid: true})
		return
	}

	// osquery reports completion via the `statuses` map. Results may be empty even on success.
	if len(req.Statuses) == 0 {
		for queryIDStr, results := range req.Queries {
			queryID, err := uuid.Parse(queryIDStr)
			if err != nil {
				slog.Error("invalid query id", "id", queryIDStr)
				continue
			}

			resJSON, err := json.Marshal(results)
			if err != nil {
				slog.Error("failed to marshal query results", "error", err)
				continue
			}
			if err := h.repo.SaveQueryResults(r.Context(), host.ID, queryID, "completed", json.RawMessage(resJSON), nil); err != nil {
				slog.Error("failed to save query results", "error", err)
				continue
			}

			h.publishQueryResultEvent(r.Context(), host.ID, queryID, pubsub.QueryResultStatusCompleted, nil)
		}

		h.jsonResponse(w, DistributedWriteResponse{})
		return
	}

	for queryIDStr, statusCode := range req.Statuses {
		queryID, err := uuid.Parse(queryIDStr)
		if err != nil {
			slog.Error("invalid query id", "id", queryIDStr)
			continue
		}

		status := "completed"
		var errorText *string
		if statusCode != 0 {
			status = "failed"
			s := fmt.Sprintf("osquery status %d", statusCode)
			errorText = &s
		}

		var resJSON json.RawMessage
		if results, ok := req.Queries[queryIDStr]; ok {
			b, err := json.Marshal(results)
			if err != nil {
				slog.Error("failed to marshal query results", "error", err)
				status = "failed"
				s := "failed to marshal query results"
				errorText = &s
				resJSON = nil
			} else {
				resJSON = json.RawMessage(b)
			}
		}

		if err := h.repo.SaveQueryResults(r.Context(), host.ID, queryID, status, resJSON, errorText); err != nil {
			slog.Error("failed to save query results", "error", err)
			continue
		}

		h.publishQueryResultEvent(r.Context(), host.ID, queryID, status, errorText)
	}

	h.jsonResponse(w, DistributedWriteResponse{})
}

func (h *Handlers) HostsPage(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	hosts, err := h.repo.ListByOrganization(r.Context(), activeOrg.ID)
	if err != nil {
		slog.Error("failed to list hosts", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pages.HostsPage("Hosts", hosts).Render(r.Context(), w)
}

func (h *Handlers) HostDetailsPage(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "id")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}

	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	host, err := h.repo.GetByIDAndOrganization(r.Context(), hostID, activeOrg.ID)
	if err != nil {
		slog.Error("failed to get host", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if host == nil {
		// Treat org mismatch as not found.
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	results, err := h.repo.GetRecentResults(r.Context(), hostID)
	if err != nil {
		slog.Error("failed to get recent results", "error", err)
	}

	pages.HostDetailsPage(host.HostIdentifier, host, results).Render(r.Context(), w)
}

func (h *Handlers) HostResultsSSE(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "id")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}

	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	host, err := h.repo.GetByIDAndOrganization(r.Context(), hostID, activeOrg.ID)
	if err != nil {
		slog.Error("failed to get host", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if host == nil {
		// Treat org mismatch as not found.
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	sse := datastar.NewSSE(w, r)

	results, err := h.repo.GetRecentResults(ctx, hostID)
	if err != nil {
		_ = sse.ConsoleError(err)
		return
	}
	if err := sse.PatchElementTempl(pages.HostResultsTable(hostIDStr, results)); err != nil {
		return
	}

	if h.pubsub == nil {
		h.pollResultsLegacy(ctx, sse, hostID, hostIDStr, results)
		return
	}

	subscriber, err := h.pubsub.NewSubscriber(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create subscriber; falling back to polling", "error", err)
		h.pollResultsLegacy(ctx, sse, hostID, hostIDStr, results)
		return
	}
	defer func() {
		_ = subscriber.Close()
	}()

	topic := pubsub.TopicQueryResults(hostID)
	messages, err := subscriber.Subscribe(ctx, topic)
	if err != nil {
		slog.ErrorContext(ctx, "failed to subscribe; falling back to polling", "error", err, "topic", topic)
		h.pollResultsLegacy(ctx, sse, hostID, hostIDStr, results)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-messages:
			if msg == nil {
				return
			}

			event, err := pubsub.ParseQueryResultEvent(msg)
			if err != nil {
				slog.ErrorContext(ctx, "failed to parse query result event", "error", err)
				msg.Nack()
				continue
			}

			// Topic-scoped, but keep it defensive.
			if event.HostID != hostID {
				msg.Ack()
				continue
			}

			results, err := h.repo.GetRecentResults(ctx, hostID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to get recent results after event", "error", err)
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

// pollResultsLegacy implements the fallback polling mechanism for HostResultsSSE.
// Used when pub/sub is unavailable or subscription fails.
func (h *Handlers) pollResultsLegacy(
	ctx context.Context,
	sse *datastar.ServerSentEventGenerator,
	hostID uuid.UUID,
	hostIDStr string,
	initialResults []services.QueryResult,
) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	last, err := json.Marshal(initialResults)
	if err != nil {
		last = nil
	}

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

			b, err := json.Marshal(results)
			if err != nil {
				_ = sse.ConsoleError(err)
				return
			}

			if bytes.Equal(b, last) {
				continue
			}
			last = b

			if err := sse.PatchElementTempl(pages.HostResultsTable(hostIDStr, results)); err != nil {
				return
			}
		}
	}
}

func (h *Handlers) RunQuery(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "id")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}

	type Store struct {
		Query string `json:"query"`
	}
	var store Store
	if err := datastar.ReadSignals(r, &store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if store.Query == "" {
		http.Error(w, "query cannot be empty", http.StatusBadRequest)
		return
	}

	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	host, err := h.repo.GetByIDAndOrganization(r.Context(), hostID, activeOrg.ID)
	if err != nil {
		slog.Error("failed to get host", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if host == nil {
		// Treat org mismatch as not found.
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	queryID, err := h.repo.QueueQuery(r.Context(), store.Query, []uuid.UUID{host.ID})
	if err != nil {
		slog.Error("failed to queue query", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("queued distributed query", "query_id", queryID, "host_id", hostID)

	// Send a script to close the dialog and maybe show a toast
	sse := datastar.NewSSE(w, r)
	if err := sse.ExecuteScript(fmt.Sprintf("document.querySelector('[data-tui-dialog-close=\"query-dialog-%s\"]').click()", hostIDStr)); err != nil {
		return
	}
}

func (h *Handlers) publishQueryResultEvent(ctx context.Context, hostID uuid.UUID, queryID uuid.UUID, status string, errorText *string) {
	if h.publisher == nil {
		return
	}

	topic := pubsub.TopicQueryResults(hostID)
	event := pubsub.QueryResultEvent{
		HostID:     hostID,
		QueryID:    queryID,
		Status:     status,
		OccurredAt: time.Now().UTC(),
		Error:      errorText,
	}

	if err := h.publisher.Publish(topic, event.ToMessage()); err != nil {
		slog.ErrorContext(ctx, "failed to publish query result event", "error", err, "topic", topic, "host_id", hostID, "query_id", queryID)
		return
	}

	slog.DebugContext(ctx, "published query result event", "topic", topic, "host_id", hostID, "query_id", queryID, "status", status)
}

func (h *Handlers) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}
