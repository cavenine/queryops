package osquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/cavenine/queryops/features/auth"
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
	QueueQuery(ctx context.Context, organizationID uuid.UUID, createdBy *int, name *string, description *string, query string, hostIDs []uuid.UUID) (uuid.UUID, error)

	GetCampaignByIDAndOrganization(ctx context.Context, campaignID uuid.UUID, organizationID uuid.UUID) (*services.Campaign, error)
	ListCampaignsByOrganization(ctx context.Context, organizationID uuid.UUID, limit int) ([]*services.Campaign, error)
	GetCampaignTargets(ctx context.Context, campaignID uuid.UUID) ([]*services.CampaignTarget, error)
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
			h.publishCampaignResultEvent(r.Context(), queryID, host, pubsub.QueryResultStatusCompleted, len(results), nil)
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

		var (
			resJSON  json.RawMessage
			rowCount int
		)
		if results, ok := req.Queries[queryIDStr]; ok {
			rowCount = len(results)
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
		h.publishCampaignResultEvent(r.Context(), queryID, host, status, rowCount, errorText)
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

func (h *Handlers) CampaignsPage(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	campaigns, err := h.repo.ListCampaignsByOrganization(r.Context(), activeOrg.ID, 50)
	if err != nil {
		slog.Error("failed to list campaigns", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pages.CampaignsPage("Live Queries", campaigns).Render(r.Context(), w)
}

func (h *Handlers) CampaignNewPage(w http.ResponseWriter, r *http.Request) {
	pages.CampaignNewPage("New Live Query").Render(r.Context(), w)
}

func (h *Handlers) RunCampaign(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type Store struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Query       string `json:"query"`
	}
	var store Store
	if err := datastar.ReadSignals(r, &store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	store.Query = strings.TrimSpace(store.Query)
	if store.Query == "" {
		http.Error(w, "query cannot be empty", http.StatusBadRequest)
		return
	}

	store.Name = strings.TrimSpace(store.Name)
	store.Description = strings.TrimSpace(store.Description)

	var (
		name        *string
		description *string
	)
	if store.Name != "" {
		s := store.Name
		name = &s
	}
	if store.Description != "" {
		s := store.Description
		description = &s
	}

	ctx := r.Context()

	var createdBy *int
	if user := auth.GetUserFromContext(ctx); user != nil {
		createdBy = &user.ID
	}

	hosts, err := h.repo.ListByOrganization(ctx, activeOrg.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list hosts", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	hostIDs := make([]uuid.UUID, 0, len(hosts))
	for _, host := range hosts {
		hostIDs = append(hostIDs, host.ID)
	}

	campaignID, err := h.repo.QueueQuery(ctx, activeOrg.ID, createdBy, name, description, store.Query, hostIDs)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create campaign", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.ExecuteScript(fmt.Sprintf("window.location = '/campaigns/%s'", campaignID.String())); err != nil {
		return
	}
}

func (h *Handlers) CampaignPage(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	campaign, err := h.repo.GetCampaignByIDAndOrganization(ctx, campaignID, activeOrg.ID)
	if err != nil {
		slog.Error("failed to get campaign", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if campaign == nil {
		http.Error(w, "campaign not found", http.StatusNotFound)
		return
	}

	targets, err := h.repo.GetCampaignTargets(ctx, campaignID)
	if err != nil {
		slog.Error("failed to get campaign targets", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	title := "Live Query"
	pages.CampaignDetailsPage(title, campaign, targets).Render(ctx, w)
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

	user := auth.GetUserFromContext(r.Context())
	var createdBy *int
	if user != nil {
		createdBy = &user.ID
	}

	queryID, err := h.repo.QueueQuery(r.Context(), activeOrg.ID, createdBy, nil, nil, store.Query, []uuid.UUID{host.ID})
	if err != nil {
		slog.Error("failed to queue query", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("queued campaign query", "campaign_id", queryID, "host_id", hostID)

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

func (h *Handlers) publishCampaignResultEvent(ctx context.Context, campaignID uuid.UUID, host *services.Host, status string, rowCount int, errorText *string) {
	if h.publisher == nil {
		return
	}
	if host == nil {
		return
	}

	topic := pubsub.TopicCampaign(campaignID)
	event := pubsub.CampaignResultEvent{
		CampaignID:     campaignID,
		HostID:         host.ID,
		HostIdentifier: host.HostIdentifier,
		Status:         status,
		OccurredAt:     time.Now().UTC(),
		RowCount:       rowCount,
		Error:          errorText,
	}

	if err := h.publisher.Publish(topic, event.ToMessage()); err != nil {
		slog.ErrorContext(ctx, "failed to publish campaign result event", "error", err, "topic", topic, "campaign_id", campaignID, "host_id", host.ID)
		return
	}

	slog.DebugContext(ctx, "published campaign result event", "topic", topic, "campaign_id", campaignID, "host_id", host.ID, "status", status)
}

type createCampaignRequest struct {
	Query       string      `json:"query"`
	Name        *string     `json:"name,omitempty"`
	Description *string     `json:"description,omitempty"`
	HostIDs     []uuid.UUID `json:"host_ids,omitempty"`
}

type createCampaignResponse struct {
	CampaignID  uuid.UUID `json:"campaign_id"`
	TargetCount int       `json:"target_count"`
}

func (h *Handlers) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Name != nil && *req.Name == "" {
		req.Name = nil
	}
	if req.Description != nil && *req.Description == "" {
		req.Description = nil
	}
	if req.Query == "" {
		http.Error(w, "query cannot be empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	var createdBy *int
	if user := auth.GetUserFromContext(ctx); user != nil {
		createdBy = &user.ID
	}

	targetHostIDs := req.HostIDs
	if len(targetHostIDs) == 0 {
		hosts, err := h.repo.ListByOrganization(ctx, activeOrg.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to list hosts", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		for _, host := range hosts {
			targetHostIDs = append(targetHostIDs, host.ID)
		}
	} else {
		for _, hostID := range targetHostIDs {
			host, err := h.repo.GetByIDAndOrganization(ctx, hostID, activeOrg.ID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to load host", "error", err, "host_id", hostID)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if host == nil {
				http.Error(w, "host not found", http.StatusNotFound)
				return
			}
		}
	}

	if len(targetHostIDs) == 0 {
		http.Error(w, "no target hosts", http.StatusBadRequest)
		return
	}

	campaignID, err := h.repo.QueueQuery(ctx, activeOrg.ID, createdBy, req.Name, req.Description, req.Query, targetHostIDs)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create campaign", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	h.jsonResponse(w, createCampaignResponse{CampaignID: campaignID, TargetCount: len(targetHostIDs)})
}

func (h *Handlers) GetCampaign(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return
	}

	campaign, err := h.repo.GetCampaignByIDAndOrganization(r.Context(), campaignID, activeOrg.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get campaign", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if campaign == nil {
		http.Error(w, "campaign not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, campaign)
}

type listCampaignsResponse struct {
	Campaigns []*services.Campaign `json:"campaigns"`
}

func (h *Handlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	campaigns, err := h.repo.ListCampaignsByOrganization(r.Context(), activeOrg.ID, 50)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list campaigns", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, listCampaignsResponse{Campaigns: campaigns})
}

func (h *Handlers) CampaignResultsSSE(w http.ResponseWriter, r *http.Request) {
	activeOrg := org.GetOrganizationFromContext(r.Context())
	if activeOrg == nil {
		slog.Error("missing active organization in context")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	campaign, err := h.repo.GetCampaignByIDAndOrganization(ctx, campaignID, activeOrg.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get campaign", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if campaign == nil {
		http.Error(w, "campaign not found", http.StatusNotFound)
		return
	}

	targets, err := h.repo.GetCampaignTargets(ctx, campaignID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get campaign targets", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(pages.CampaignResultsTable(campaignID.String(), campaign, targets)); err != nil {
		return
	}

	if campaign.Status == "completed" || campaign.Status == "failed" {
		return
	}

	if h.pubsub == nil {
		h.pollCampaignLegacy(ctx, sse, activeOrg.ID, campaignID, campaign, targets)
		return
	}

	subscriber, err := h.pubsub.NewSubscriber(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create subscriber; falling back to polling", "error", err)
		h.pollCampaignLegacy(ctx, sse, activeOrg.ID, campaignID, campaign, targets)
		return
	}
	defer func() {
		_ = subscriber.Close()
	}()

	topic := pubsub.TopicCampaign(campaignID)
	messages, err := subscriber.Subscribe(ctx, topic)
	if err != nil {
		slog.ErrorContext(ctx, "failed to subscribe; falling back to polling", "error", err, "topic", topic)
		h.pollCampaignLegacy(ctx, sse, activeOrg.ID, campaignID, campaign, targets)
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

			event, err := pubsub.ParseCampaignResultEvent(msg)
			if err != nil {
				slog.ErrorContext(ctx, "failed to parse campaign result event", "error", err)
				msg.Nack()
				continue
			}

			if event.CampaignID != campaignID {
				msg.Ack()
				continue
			}

			campaign, err = h.repo.GetCampaignByIDAndOrganization(ctx, campaignID, activeOrg.ID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to get campaign", "error", err)
				msg.Nack()
				continue
			}
			if campaign == nil {
				msg.Ack()
				return
			}

			targets, err = h.repo.GetCampaignTargets(ctx, campaignID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to get campaign targets", "error", err)
				msg.Nack()
				continue
			}

			if err := sse.PatchElementTempl(pages.CampaignResultsTable(campaignID.String(), campaign, targets)); err != nil {
				msg.Nack()
				return
			}

			msg.Ack()

			if campaign.Status == "completed" || campaign.Status == "failed" {
				return
			}
		}
	}
}

func (h *Handlers) pollCampaignLegacy(
	ctx context.Context,
	sse *datastar.ServerSentEventGenerator,
	organizationID uuid.UUID,
	campaignID uuid.UUID,
	initialCampaign *services.Campaign,
	initialTargets []*services.CampaignTarget,
) {
	snapshot, err := json.Marshal(map[string]any{"campaign": initialCampaign, "targets": initialTargets})
	if err != nil {
		snapshot = nil
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			campaign, err := h.repo.GetCampaignByIDAndOrganization(ctx, campaignID, organizationID)
			if err != nil {
				_ = sse.ConsoleError(err)
				return
			}
			if campaign == nil {
				return
			}

			targets, err := h.repo.GetCampaignTargets(ctx, campaignID)
			if err != nil {
				_ = sse.ConsoleError(err)
				return
			}

			b, err := json.Marshal(map[string]any{"campaign": campaign, "targets": targets})
			if err != nil {
				_ = sse.ConsoleError(err)
				return
			}

			if !bytes.Equal(b, snapshot) {
				snapshot = b
				if err := sse.PatchElementTempl(pages.CampaignResultsTable(campaignID.String(), campaign, targets)); err != nil {
					return
				}
			}

			if campaign.Status == "completed" || campaign.Status == "failed" {
				return
			}
		}
	}
}

func (h *Handlers) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}
