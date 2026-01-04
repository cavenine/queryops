package osquery_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cavenine/queryops/features/auth"
	"github.com/cavenine/queryops/features/auth/services"
	"github.com/cavenine/queryops/features/organization"
	orgServices "github.com/cavenine/queryops/features/organization/services"
	"github.com/cavenine/queryops/features/osquery"
	osqueryServices "github.com/cavenine/queryops/features/osquery/services"
	"github.com/cavenine/queryops/internal/pubsub"
	"github.com/cavenine/queryops/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type noopEnrollOrgLookup struct{}

func (noopEnrollOrgLookup) GetOrganizationByEnrollSecret(context.Context, string) (*orgServices.Organization, error) {
	return nil, nil
}

func TestCampaignResultsSSE_EmitsUpdatesOnPublish(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	// Minimal user+org+host setup.
	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "sse@example.com", "x").Scan(&userID); err != nil {
		t.Fatalf("creating user: %v", err)
	}

	var orgID uuid.UUID
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO organizations (name) VALUES ($1) RETURNING id`, "sse-org").Scan(&orgID); err != nil {
		t.Fatalf("creating org: %v", err)
	}

	var hostID uuid.UUID
	if err := tdb.Pool.QueryRow(ctx, `
		INSERT INTO hosts (organization_id, host_identifier, node_key)
		VALUES ($1, $2, $3)
		RETURNING id
	`, orgID, "host-1", uuid.NewString()).Scan(&hostID); err != nil {
		t.Fatalf("creating host: %v", err)
	}

	repo := osqueryServices.NewHostRepository(tdb.Pool)
	campaignID, err := repo.QueueQuery(ctx, orgID, &userID, nil, nil, "select 1", []uuid.UUID{hostID})
	if err != nil {
		t.Fatalf("QueueQuery: %v", err)
	}

	ps, err := pubsub.New(ctx, nil) // nil config = use embedded NATS
	if err != nil {
		t.Fatalf("creating pubsub: %v", err)
	}
	defer func() { _ = ps.Close() }()

	h := osquery.NewHandlers(repo, noopEnrollOrgLookup{}, ps.Publisher(), ps)

	activeOrg := &orgServices.Organization{ID: orgID, Name: "sse-org"}
	user := &services.User{ID: userID, Email: "sse@example.com"}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.SetUserInContext(r.Context(), user)
			ctx = organization.SetOrganizationInContext(ctx, activeOrg)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Get("/api/v1/campaigns/{id}/results", h.CampaignResultsSSE)

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/campaigns/"+campaignID.String()+"/results", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	readEventLine := func(timeout time.Duration) (string, error) {
		type readResult struct {
			line string
			err  error
		}

		ch := make(chan readResult, 1)
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					ch <- readResult{"", err}
					return
				}
				if strings.HasPrefix(line, "event:") {
					ch <- readResult{line, nil}
					return
				}
			}
		}()

		select {
		case res := <-ch:
			return res.line, res.err
		case <-time.After(timeout):
			return "", context.DeadlineExceeded
		}
	}

	// Initial patch.
	line, err := readEventLine(5 * time.Second)
	if err != nil {
		t.Fatalf("reading initial event line: %v", err)
	}
	if !strings.Contains(line, "datastar-patch-elements") {
		t.Fatalf("initial event line = %q", line)
	}

	event := pubsub.CampaignResultEvent{
		CampaignID:     campaignID,
		HostID:         hostID,
		HostIdentifier: "host-1",
		Status:         pubsub.QueryResultStatusCompleted,
		OccurredAt:     time.Now().UTC(),
		RowCount:       1,
	}
	if err := ps.Publisher().Publish(pubsub.TopicCampaign(campaignID), event.ToMessage()); err != nil {
		t.Fatalf("publishing event: %v", err)
	}

	line, err = readEventLine(5 * time.Second)
	if err != nil {
		t.Fatalf("reading next event line: %v", err)
	}
	if !strings.Contains(line, "datastar-patch-elements") {
		t.Fatalf("next event line = %q", line)
	}
}
