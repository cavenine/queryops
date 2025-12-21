package osquery_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"

	orgServices "github.com/cavenine/queryops/features/organization/services"
	"github.com/cavenine/queryops/features/osquery"
	osqueryServices "github.com/cavenine/queryops/features/osquery/services"
)

type stubHostRepo struct {
	EnrollFunc                func(ctx context.Context, hostIdentifier string, hostDetails json.RawMessage, organizationID uuid.UUID) (string, error)
	GetByNodeKeyFunc          func(ctx context.Context, nodeKey string) (*osqueryServices.Host, error)
	UpdateLastConfigFunc      func(ctx context.Context, nodeKey string) error
	UpdateLastLoggerFunc      func(ctx context.Context, nodeKey string) error
	UpdateLastDistributedFunc func(ctx context.Context, nodeKey string) error
	GetConfigForHostFunc      func(ctx context.Context, nodeKey string) (json.RawMessage, error)
	SaveResultLogsFunc        func(ctx context.Context, hostID uuid.UUID, name, action string, columns json.RawMessage, timestamp time.Time) error
	SaveStatusLogsFunc        func(ctx context.Context, hostID uuid.UUID, line int, message string, severity int, filename string, createdAt time.Time) error
	GetPendingQueriesFunc     func(ctx context.Context, hostID uuid.UUID) (map[string]string, error)
	SaveQueryResultsFunc      func(ctx context.Context, hostID uuid.UUID, queryID uuid.UUID, status string, results json.RawMessage, errorText *string) error

	ListByOrganizationFunc     func(ctx context.Context, organizationID uuid.UUID) ([]*osqueryServices.Host, error)
	GetByIDAndOrganizationFunc func(ctx context.Context, id uuid.UUID, organizationID uuid.UUID) (*osqueryServices.Host, error)
	GetRecentResultsFunc       func(ctx context.Context, hostID uuid.UUID) ([]osqueryServices.QueryResult, error)
	QueueQueryFunc             func(ctx context.Context, organizationID uuid.UUID, createdBy *int, name *string, description *string, query string, hostIDs []uuid.UUID) (uuid.UUID, error)

	GetCampaignByIDAndOrganizationFunc func(ctx context.Context, campaignID uuid.UUID, organizationID uuid.UUID) (*osqueryServices.Campaign, error)
	ListCampaignsByOrganizationFunc    func(ctx context.Context, organizationID uuid.UUID, limit int) ([]*osqueryServices.Campaign, error)
	GetCampaignTargetsFunc             func(ctx context.Context, campaignID uuid.UUID) ([]*osqueryServices.CampaignTarget, error)
}

func (s *stubHostRepo) Enroll(ctx context.Context, hostIdentifier string, hostDetails json.RawMessage, organizationID uuid.UUID) (string, error) {
	if s.EnrollFunc == nil {
		return "", nil
	}
	return s.EnrollFunc(ctx, hostIdentifier, hostDetails, organizationID)
}

func (s *stubHostRepo) GetByNodeKey(ctx context.Context, nodeKey string) (*osqueryServices.Host, error) {
	if s.GetByNodeKeyFunc == nil {
		return nil, nil
	}
	return s.GetByNodeKeyFunc(ctx, nodeKey)
}

func (s *stubHostRepo) UpdateLastConfig(ctx context.Context, nodeKey string) error {
	if s.UpdateLastConfigFunc == nil {
		return nil
	}
	return s.UpdateLastConfigFunc(ctx, nodeKey)
}

func (s *stubHostRepo) UpdateLastLogger(ctx context.Context, nodeKey string) error {
	if s.UpdateLastLoggerFunc == nil {
		return nil
	}
	return s.UpdateLastLoggerFunc(ctx, nodeKey)
}

func (s *stubHostRepo) UpdateLastDistributed(ctx context.Context, nodeKey string) error {
	if s.UpdateLastDistributedFunc == nil {
		return nil
	}
	return s.UpdateLastDistributedFunc(ctx, nodeKey)
}

func (s *stubHostRepo) GetConfigForHost(ctx context.Context, nodeKey string) (json.RawMessage, error) {
	if s.GetConfigForHostFunc == nil {
		return nil, nil
	}
	return s.GetConfigForHostFunc(ctx, nodeKey)
}

func (s *stubHostRepo) SaveResultLogs(ctx context.Context, hostID uuid.UUID, name, action string, columns json.RawMessage, timestamp time.Time) error {
	if s.SaveResultLogsFunc == nil {
		return nil
	}
	return s.SaveResultLogsFunc(ctx, hostID, name, action, columns, timestamp)
}

func (s *stubHostRepo) SaveStatusLogs(ctx context.Context, hostID uuid.UUID, line int, message string, severity int, filename string, createdAt time.Time) error {
	if s.SaveStatusLogsFunc == nil {
		return nil
	}
	return s.SaveStatusLogsFunc(ctx, hostID, line, message, severity, filename, createdAt)
}

func (s *stubHostRepo) GetPendingQueries(ctx context.Context, hostID uuid.UUID) (map[string]string, error) {
	if s.GetPendingQueriesFunc == nil {
		return map[string]string{}, nil
	}
	return s.GetPendingQueriesFunc(ctx, hostID)
}

func (s *stubHostRepo) SaveQueryResults(ctx context.Context, hostID uuid.UUID, queryID uuid.UUID, status string, results json.RawMessage, errorText *string) error {
	if s.SaveQueryResultsFunc == nil {
		return nil
	}
	return s.SaveQueryResultsFunc(ctx, hostID, queryID, status, results, errorText)
}

func (s *stubHostRepo) ListByOrganization(ctx context.Context, organizationID uuid.UUID) ([]*osqueryServices.Host, error) {
	if s.ListByOrganizationFunc == nil {
		return nil, nil
	}
	return s.ListByOrganizationFunc(ctx, organizationID)
}

func (s *stubHostRepo) GetByIDAndOrganization(ctx context.Context, id uuid.UUID, organizationID uuid.UUID) (*osqueryServices.Host, error) {
	if s.GetByIDAndOrganizationFunc == nil {
		return nil, nil
	}
	return s.GetByIDAndOrganizationFunc(ctx, id, organizationID)
}

func (s *stubHostRepo) GetRecentResults(ctx context.Context, hostID uuid.UUID) ([]osqueryServices.QueryResult, error) {
	if s.GetRecentResultsFunc == nil {
		return nil, nil
	}
	return s.GetRecentResultsFunc(ctx, hostID)
}

func (s *stubHostRepo) QueueQuery(ctx context.Context, organizationID uuid.UUID, createdBy *int, name *string, description *string, query string, hostIDs []uuid.UUID) (uuid.UUID, error) {
	if s.QueueQueryFunc == nil {
		return uuid.Nil, nil
	}
	return s.QueueQueryFunc(ctx, organizationID, createdBy, name, description, query, hostIDs)
}

func (s *stubHostRepo) GetCampaignByIDAndOrganization(ctx context.Context, campaignID uuid.UUID, organizationID uuid.UUID) (*osqueryServices.Campaign, error) {
	if s.GetCampaignByIDAndOrganizationFunc == nil {
		return nil, nil
	}
	return s.GetCampaignByIDAndOrganizationFunc(ctx, campaignID, organizationID)
}

func (s *stubHostRepo) ListCampaignsByOrganization(ctx context.Context, organizationID uuid.UUID, limit int) ([]*osqueryServices.Campaign, error) {
	if s.ListCampaignsByOrganizationFunc == nil {
		return nil, nil
	}
	return s.ListCampaignsByOrganizationFunc(ctx, organizationID, limit)
}

func (s *stubHostRepo) GetCampaignTargets(ctx context.Context, campaignID uuid.UUID) ([]*osqueryServices.CampaignTarget, error) {
	if s.GetCampaignTargetsFunc == nil {
		return nil, nil
	}
	return s.GetCampaignTargetsFunc(ctx, campaignID)
}

type mockPublisher struct {
	mu           sync.Mutex
	publishErr   error
	publishCalls []publishCall
}

type publishCall struct {
	topic    string
	messages []*message.Message
}

func (m *mockPublisher) Publish(topic string, messages ...*message.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyMsgs := make([]*message.Message, len(messages))
	copy(copyMsgs, messages)
	m.publishCalls = append(m.publishCalls, publishCall{topic: topic, messages: copyMsgs})
	return m.publishErr
}

func (m *mockPublisher) Close() error { return nil }

type stubEnrollOrgLookup struct {
	GetOrganizationByEnrollSecretFunc func(ctx context.Context, secret string) (*orgServices.Organization, error)
}

func (s *stubEnrollOrgLookup) GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*orgServices.Organization, error) {
	if s.GetOrganizationByEnrollSecretFunc == nil {
		return nil, nil
	}
	return s.GetOrganizationByEnrollSecretFunc(ctx, secret)
}

func TestEnroll(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name       string
		body       string
		setup      func(repo *stubHostRepo, orgLookup *stubEnrollOrgLookup)
		wantStatus int
		wantResp   *osquery.EnrollmentResponse
	}{
		{
			name:       "invalid json",
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unknown enroll secret",
			body: `{"enroll_secret":"bad","host_identifier":"h1","host_details":{}}`,
			setup: func(_ *stubHostRepo, orgLookup *stubEnrollOrgLookup) {
				orgLookup.GetOrganizationByEnrollSecretFunc = func(_ context.Context, secret string) (*orgServices.Organization, error) {
					if secret != "bad" {
						t.Fatalf("secret = %q", secret)
					}
					return nil, orgServices.ErrOrganizationNotFound
				}
			},
			wantStatus: http.StatusOK,
			wantResp:   &osquery.EnrollmentResponse{NodeInvalid: true},
		},
		{
			name: "org lookup internal error",
			body: `{"enroll_secret":"x","host_identifier":"h1","host_details":{}}`,
			setup: func(_ *stubHostRepo, orgLookup *stubEnrollOrgLookup) {
				orgLookup.GetOrganizationByEnrollSecretFunc = func(context.Context, string) (*orgServices.Organization, error) {
					return nil, errors.New("boom")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "success",
			body: `{"enroll_secret":"good","host_identifier":"h1","host_details":{"platform":"linux"}}`,
			setup: func(repo *stubHostRepo, orgLookup *stubEnrollOrgLookup) {
				orgLookup.GetOrganizationByEnrollSecretFunc = func(_ context.Context, secret string) (*orgServices.Organization, error) {
					if secret != "good" {
						t.Fatalf("secret = %q", secret)
					}
					return &orgServices.Organization{ID: orgID, Name: "org"}, nil
				}
				repo.EnrollFunc = func(_ context.Context, hostIdentifier string, hostDetails json.RawMessage, organizationID uuid.UUID) (string, error) {
					if hostIdentifier != "h1" {
						t.Fatalf("hostIdentifier = %q", hostIdentifier)
					}
					if organizationID != orgID {
						t.Fatalf("organizationID = %s", organizationID)
					}
					var got map[string]string
					if err := json.Unmarshal(hostDetails, &got); err != nil {
						t.Fatalf("unmarshal hostDetails: %v", err)
					}
					if got["platform"] != "linux" {
						t.Fatalf("hostDetails = %#v", got)
					}
					return "node-key", nil
				}
			},
			wantStatus: http.StatusOK,
			wantResp:   &osquery.EnrollmentResponse{NodeKey: "node-key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubHostRepo{}
			orgLookup := &stubEnrollOrgLookup{}
			if tt.setup != nil {
				tt.setup(repo, orgLookup)
			}

			h := osquery.NewHandlers(repo, orgLookup, nil, nil)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/osquery/enroll", strings.NewReader(tt.body))
			h.Enroll(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantResp == nil {
				return
			}

			var got osquery.EnrollmentResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if got != *tt.wantResp {
				t.Fatalf("response = %#v, want %#v", got, *tt.wantResp)
			}
		})
	}
}

func TestConfig(t *testing.T) {
	hostID := uuid.New()

	tests := []struct {
		name       string
		body       string
		setup      func(repo *stubHostRepo)
		wantStatus int
		wantResp   *osquery.ConfigResponse
	}{
		{
			name:       "invalid json",
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unknown node key",
			body: `{"node_key":"missing"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return nil, nil
				}
			},
			wantStatus: http.StatusOK,
			wantResp:   &osquery.ConfigResponse{NodeInvalid: true},
		},
		{
			name: "update last config error still returns config",
			body: `{"node_key":"k1"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return &osqueryServices.Host{ID: hostID}, nil
				}
				repo.UpdateLastConfigFunc = func(context.Context, string) error {
					return errors.New("fail")
				}
				repo.GetConfigForHostFunc = func(context.Context, string) (json.RawMessage, error) {
					return json.RawMessage(`{"options":{"foo":1}}`), nil
				}
			},
			wantStatus: http.StatusOK,
			wantResp: &osquery.ConfigResponse{
				Options: map[string]any{"foo": float64(1)},
			},
		},
		{
			name: "get config error",
			body: `{"node_key":"k1"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return &osqueryServices.Host{ID: hostID}, nil
				}
				repo.GetConfigForHostFunc = func(context.Context, string) (json.RawMessage, error) {
					return nil, errors.New("db")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "invalid config json",
			body: `{"node_key":"k1"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return &osqueryServices.Host{ID: hostID}, nil
				}
				repo.GetConfigForHostFunc = func(context.Context, string) (json.RawMessage, error) {
					return json.RawMessage("not-json"), nil
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubHostRepo{}
			if tt.setup != nil {
				tt.setup(repo)
			}

			h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, nil, nil)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/osquery/config", strings.NewReader(tt.body))
			h.Config(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantResp == nil {
				return
			}

			var got osquery.ConfigResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if got.NodeInvalid != tt.wantResp.NodeInvalid {
				t.Fatalf("node_invalid = %v, want %v", got.NodeInvalid, tt.wantResp.NodeInvalid)
			}
			if tt.wantResp.Options != nil {
				if got.Options["foo"] != tt.wantResp.Options["foo"] {
					t.Fatalf("options.foo = %#v, want %#v", got.Options["foo"], tt.wantResp.Options["foo"])
				}
			}
		})
	}
}

func TestLogger_ResultLogs(t *testing.T) {
	hostID := uuid.New()

	calls := struct {
		updateLogger int
		resultLogs   int
	}{}

	repo := &stubHostRepo{}
	repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
		return &osqueryServices.Host{ID: hostID, HostIdentifier: "h1"}, nil
	}
	repo.UpdateLastLoggerFunc = func(context.Context, string) error {
		calls.updateLogger++
		return nil
	}
	repo.SaveResultLogsFunc = func(_ context.Context, gotHostID uuid.UUID, name, action string, columns json.RawMessage, ts time.Time) error {
		calls.resultLogs++
		if gotHostID != hostID {
			t.Fatalf("hostID = %s", gotHostID)
		}
		if name != "pack_test" {
			t.Fatalf("name = %q", name)
		}
		if action != "added" {
			t.Fatalf("action = %q", action)
		}
		if ts.Unix() != 10 {
			t.Fatalf("timestamp = %v", ts)
		}
		var gotCols map[string]string
		if err := json.Unmarshal(columns, &gotCols); err != nil {
			t.Fatalf("unmarshal columns: %v", err)
		}
		if gotCols["a"] != "b" {
			t.Fatalf("columns = %#v", gotCols)
		}
		return nil
	}

	h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, nil, nil)

	body := `{
		"node_key":"k1",
		"log_type":"result",
		"data":[
			{"name":"pack_test","hostIdentifier":"h1","calendarTime":"now","unixTime":10,"action":"added","columns":{"a":"b"}}
		]
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/osquery/logger", strings.NewReader(body))
	h.Logger(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if calls.updateLogger != 1 {
		t.Fatalf("updateLogger calls = %d", calls.updateLogger)
	}
	if calls.resultLogs != 1 {
		t.Fatalf("resultLogs calls = %d", calls.resultLogs)
	}
}

func TestLogger_StatusLogs(t *testing.T) {
	hostID := uuid.New()

	calls := struct {
		updateLogger int
		statusLogs   int
	}{}

	repo := &stubHostRepo{}
	repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
		return &osqueryServices.Host{ID: hostID, HostIdentifier: "h1"}, nil
	}
	repo.UpdateLastLoggerFunc = func(context.Context, string) error {
		calls.updateLogger++
		return nil
	}
	repo.SaveStatusLogsFunc = func(_ context.Context, gotHostID uuid.UUID, line int, message string, severity int, filename string, createdAt time.Time) error {
		calls.statusLogs++
		if gotHostID != hostID {
			t.Fatalf("hostID = %s", gotHostID)
		}
		if line != 123 {
			t.Fatalf("line = %d", line)
		}
		if severity != 2 {
			t.Fatalf("severity = %d", severity)
		}
		if filename != "file.cpp" {
			t.Fatalf("filename = %q", filename)
		}
		if message != "oops" {
			t.Fatalf("message = %q", message)
		}
		if createdAt.Unix() != 11 {
			t.Fatalf("createdAt = %v", createdAt)
		}
		return nil
	}

	h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, nil, nil)

	body := `{
		"node_key":"k1",
		"log_type":"status",
		"data":[
			{"line":123,"message":"oops","severity":2,"filename":"file.cpp","calendarTime":"now","unixTime":11}
		]
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/osquery/logger", strings.NewReader(body))
	h.Logger(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if calls.updateLogger != 1 {
		t.Fatalf("updateLogger calls = %d", calls.updateLogger)
	}
	if calls.statusLogs != 1 {
		t.Fatalf("statusLogs calls = %d", calls.statusLogs)
	}
}

func TestDistributedRead(t *testing.T) {
	hostID := uuid.New()

	tests := []struct {
		name       string
		body       string
		setup      func(repo *stubHostRepo)
		wantStatus int
		wantResp   *osquery.DistributedReadResponse
	}{
		{
			name:       "invalid json",
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid node key",
			body: `{"node_key":"bad"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return nil, nil
				}
			},
			wantStatus: http.StatusOK,
			wantResp:   &osquery.DistributedReadResponse{NodeInvalid: true, Queries: map[string]string{}},
		},
		{
			name: "pending query error returns empty map",
			body: `{"node_key":"k1"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return &osqueryServices.Host{ID: hostID}, nil
				}
				repo.GetPendingQueriesFunc = func(context.Context, uuid.UUID) (map[string]string, error) {
					return nil, errors.New("db")
				}
			},
			wantStatus: http.StatusOK,
			wantResp:   &osquery.DistributedReadResponse{Queries: map[string]string{}},
		},
		{
			name: "success",
			body: `{"node_key":"k1"}`,
			setup: func(repo *stubHostRepo) {
				repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
					return &osqueryServices.Host{ID: hostID}, nil
				}
				repo.GetPendingQueriesFunc = func(context.Context, uuid.UUID) (map[string]string, error) {
					return map[string]string{"q": "select 1"}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantResp:   &osquery.DistributedReadResponse{Queries: map[string]string{"q": "select 1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubHostRepo{}
			if tt.setup != nil {
				tt.setup(repo)
			}

			h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, nil, nil)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/osquery/distributed_read", strings.NewReader(tt.body))
			h.DistributedRead(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantResp == nil {
				return
			}

			var got osquery.DistributedReadResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if got.NodeInvalid != tt.wantResp.NodeInvalid {
				t.Fatalf("node_invalid = %v, want %v", got.NodeInvalid, tt.wantResp.NodeInvalid)
			}
			if len(got.Queries) != len(tt.wantResp.Queries) {
				t.Fatalf("queries = %#v, want %#v", got.Queries, tt.wantResp.Queries)
			}
			for k, v := range tt.wantResp.Queries {
				if got.Queries[k] != v {
					t.Fatalf("queries[%q] = %q, want %q", k, got.Queries[k], v)
				}
			}
		})
	}
}

func TestDistributedWrite(t *testing.T) {
	hostID := uuid.New()
	q1 := uuid.New()
	q2 := uuid.New()

	var calls []struct {
		queryID   uuid.UUID
		status    string
		results   json.RawMessage
		errorText *string
	}

	repo := &stubHostRepo{}
	repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
		return &osqueryServices.Host{ID: hostID}, nil
	}
	repo.SaveQueryResultsFunc = func(_ context.Context, _ uuid.UUID, queryID uuid.UUID, status string, results json.RawMessage, errorText *string) error {
		calls = append(calls, struct {
			queryID   uuid.UUID
			status    string
			results   json.RawMessage
			errorText *string
		}{queryID: queryID, status: status, results: results, errorText: errorText})
		return nil
	}

	h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, nil, nil)

	body := `{
		"node_key":"k1",
		"queries":{
			"` + q1.String() + `":[{"a":"b"}],
			"` + q2.String() + `":[]
		},
		"statuses":{
			"` + q1.String() + `":0,
			"` + q2.String() + `":1
		}
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/osquery/distributed_write", strings.NewReader(body))
	h.DistributedWrite(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if len(calls) != 2 {
		t.Fatalf("SaveQueryResults calls = %d", len(calls))
	}

	for _, c := range calls {
		switch c.queryID {
		case q1:
			if c.status != "completed" {
				t.Fatalf("q1 status = %q", c.status)
			}
			if c.errorText != nil {
				t.Fatalf("q1 errorText = %v", *c.errorText)
			}
			if len(c.results) == 0 {
				t.Fatalf("q1 results empty")
			}
		case q2:
			if c.status != "failed" {
				t.Fatalf("q2 status = %q", c.status)
			}
			if c.errorText == nil || *c.errorText != "osquery status 1" {
				if c.errorText == nil {
					t.Fatalf("q2 errorText = nil")
				}
				t.Fatalf("q2 errorText = %q", *c.errorText)
			}
		default:
			t.Fatalf("unexpected queryID %s", c.queryID)
		}
	}
}
