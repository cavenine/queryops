package osquery

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/features/osquery/services"
)

type Handlers struct {
	repo         *services.HostRepository
	enrollSecret string
}

func NewHandlers(repo *services.HostRepository) *Handlers {
	return &Handlers{
		repo:         repo,
		enrollSecret: config.Global.OsqueryEnrollSecret,
	}
}

func (h *Handlers) Enroll(w http.ResponseWriter, r *http.Request) {
	var req EnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.EnrollSecret != h.enrollSecret {
		slog.Warn("invalid enrollment secret", "received", req.EnrollSecret)
		h.jsonResponse(w, EnrollmentResponse{NodeInvalid: true})
		return
	}

	nodeKey, err := h.repo.Enroll(r.Context(), req.HostIdentifier, req.HostDetails)
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

	// Hardcoded sample config for now
	resp := ConfigResponse{
		Schedule: map[string]ScheduledQuery{
			"processes": {
				Query:    "SELECT * FROM processes;",
				Interval: 60,
			},
			"os_version": {
				Query:    "SELECT * FROM os_version;",
				Interval: 3600,
			},
		},
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
		h.jsonResponse(w, DistributedReadResponse{NodeInvalid: true})
		return
	}

	if err := h.repo.UpdateLastDistributed(r.Context(), req.NodeKey); err != nil {
		slog.Error("failed to update last distributed", "error", err)
	}

	// No distributed queries for now
	h.jsonResponse(w, DistributedReadResponse{
		Queries: map[string]string{},
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

	slog.Info("received distributed query results", "host_identifier", host.HostIdentifier, "query_count", len(req.Queries))

	h.jsonResponse(w, DistributedWriteResponse{})
}

func (h *Handlers) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}
