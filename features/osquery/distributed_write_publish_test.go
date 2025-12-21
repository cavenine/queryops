package osquery_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cavenine/queryops/features/osquery"
	osqueryServices "github.com/cavenine/queryops/features/osquery/services"
	"github.com/cavenine/queryops/internal/pubsub"
	"github.com/google/uuid"
)

func TestDistributedWrite_PublishesEventOnSuccess(t *testing.T) {
	hostID := uuid.New()
	queryID := uuid.New()

	repo := &stubHostRepo{}
	repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
		return &osqueryServices.Host{ID: hostID}, nil
	}
	repo.SaveQueryResultsFunc = func(ctx context.Context, gotHostID uuid.UUID, gotQueryID uuid.UUID, status string, results json.RawMessage, errorText *string) error {
		if gotHostID != hostID {
			t.Fatalf("hostID = %s, want %s", gotHostID, hostID)
		}
		if gotQueryID != queryID {
			t.Fatalf("queryID = %s, want %s", gotQueryID, queryID)
		}
		if status != pubsub.QueryResultStatusCompleted {
			t.Fatalf("status = %q", status)
		}
		return nil
	}

	publisher := &mockPublisher{}
	h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, publisher, nil)

	bodyStruct := osquery.DistributedWriteRequest{
		NodeKey: "k1",
		Statuses: map[string]int{
			queryID.String(): 0,
		},
		Queries: map[string][]map[string]string{
			queryID.String(): {},
		},
	}
	body, err := json.Marshal(bodyStruct)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/osquery/distributed_write", strings.NewReader(string(body)))
	h.DistributedWrite(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	publisher.mu.Lock()
	calls := append([]publishCall(nil), publisher.publishCalls...)
	publisher.mu.Unlock()

	if len(calls) != 2 {
		t.Fatalf("publish calls = %d, want 2", len(calls))
	}

	callsByTopic := map[string]publishCall{}
	for _, call := range calls {
		callsByTopic[call.topic] = call
	}

	wantHostTopic := pubsub.TopicQueryResults(hostID)
	hostCall, ok := callsByTopic[wantHostTopic]
	if !ok {
		t.Fatalf("missing publish call for topic %q", wantHostTopic)
	}
	if len(hostCall.messages) != 1 {
		t.Fatalf("published messages = %d, want 1", len(hostCall.messages))
	}

	hostEvent, err := pubsub.ParseQueryResultEvent(hostCall.messages[0])
	if err != nil {
		t.Fatalf("ParseQueryResultEvent: %v", err)
	}
	if hostEvent.HostID != hostID {
		t.Fatalf("event.HostID = %s, want %s", hostEvent.HostID, hostID)
	}
	if hostEvent.QueryID != queryID {
		t.Fatalf("event.QueryID = %s, want %s", hostEvent.QueryID, queryID)
	}
	if hostEvent.Status != pubsub.QueryResultStatusCompleted {
		t.Fatalf("event.Status = %q, want %q", hostEvent.Status, pubsub.QueryResultStatusCompleted)
	}
	if time.Since(hostEvent.OccurredAt) > time.Minute {
		t.Fatalf("event.OccurredAt looks too old: %v", hostEvent.OccurredAt)
	}

	wantCampaignTopic := pubsub.TopicCampaign(queryID)
	campaignCall, ok := callsByTopic[wantCampaignTopic]
	if !ok {
		t.Fatalf("missing publish call for topic %q", wantCampaignTopic)
	}
	if len(campaignCall.messages) != 1 {
		t.Fatalf("published messages = %d, want 1", len(campaignCall.messages))
	}

	campaignEvent, err := pubsub.ParseCampaignResultEvent(campaignCall.messages[0])
	if err != nil {
		t.Fatalf("ParseCampaignResultEvent: %v", err)
	}
	if campaignEvent.CampaignID != queryID {
		t.Fatalf("event.CampaignID = %s, want %s", campaignEvent.CampaignID, queryID)
	}
	if campaignEvent.HostID != hostID {
		t.Fatalf("event.HostID = %s, want %s", campaignEvent.HostID, hostID)
	}
	if campaignEvent.Status != pubsub.QueryResultStatusCompleted {
		t.Fatalf("event.Status = %q, want %q", campaignEvent.Status, pubsub.QueryResultStatusCompleted)
	}
	if time.Since(campaignEvent.OccurredAt) > time.Minute {
		t.Fatalf("event.OccurredAt looks too old: %v", campaignEvent.OccurredAt)
	}
}

func TestDistributedWrite_SkipsPublishOnSaveFailure(t *testing.T) {
	hostID := uuid.New()
	queryID := uuid.New()

	repo := &stubHostRepo{}
	repo.GetByNodeKeyFunc = func(context.Context, string) (*osqueryServices.Host, error) {
		return &osqueryServices.Host{ID: hostID}, nil
	}
	repo.SaveQueryResultsFunc = func(context.Context, uuid.UUID, uuid.UUID, string, json.RawMessage, *string) error {
		return errors.New("db")
	}

	publisher := &mockPublisher{}
	h := osquery.NewHandlers(repo, &stubEnrollOrgLookup{}, publisher, nil)

	bodyStruct := osquery.DistributedWriteRequest{
		NodeKey: "k1",
		Statuses: map[string]int{
			queryID.String(): 0,
		},
		Queries: map[string][]map[string]string{
			queryID.String(): {},
		},
	}
	body, err := json.Marshal(bodyStruct)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/osquery/distributed_write", strings.NewReader(string(body)))
	h.DistributedWrite(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	publisher.mu.Lock()
	calls := len(publisher.publishCalls)
	publisher.mu.Unlock()

	if calls != 0 {
		t.Fatalf("publish calls = %d, want 0", calls)
	}
}
