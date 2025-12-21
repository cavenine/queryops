package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cavenine/queryops/features/osquery/services"
	"github.com/cavenine/queryops/internal/testdb"
	"github.com/google/uuid"
)

func TestCampaignRepository_Flow(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "campaign@example.com", "x").Scan(&userID); err != nil {
		t.Fatalf("creating user: %v", err)
	}

	var orgID uuid.UUID
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO organizations (name) VALUES ($1) RETURNING id`, "campaign-org").Scan(&orgID); err != nil {
		t.Fatalf("creating org: %v", err)
	}

	insertHost := func(hostIdentifier string) uuid.UUID {
		t.Helper()
		var hostID uuid.UUID
		err := tdb.Pool.QueryRow(ctx, `
			INSERT INTO hosts (organization_id, host_identifier, node_key)
			VALUES ($1, $2, $3)
			RETURNING id
		`, orgID, hostIdentifier, uuid.NewString()).Scan(&hostID)
		if err != nil {
			t.Fatalf("creating host %q: %v", hostIdentifier, err)
		}
		return hostID
	}

	hostA := insertHost("host-a")
	hostB := insertHost("host-b")

	repo := services.NewHostRepository(tdb.Pool)

	name := "Test campaign"
	description := "Runs a query on hosts"
	createdBy := userID

	campaignID, err := repo.QueueQuery(ctx, orgID, &createdBy, &name, &description, "select 1", []uuid.UUID{hostA, hostB})
	if err != nil {
		t.Fatalf("QueueQuery: %v", err)
	}

	campaign, err := repo.GetCampaignByIDAndOrganization(ctx, campaignID, orgID)
	if err != nil {
		t.Fatalf("GetCampaignByIDAndOrganization: %v", err)
	}
	if campaign == nil {
		t.Fatalf("expected campaign")
	}
	if campaign.TargetCount != 2 {
		t.Fatalf("TargetCount = %d, want 2", campaign.TargetCount)
	}
	if campaign.ResultCount != 0 {
		t.Fatalf("ResultCount = %d, want 0", campaign.ResultCount)
	}
	if campaign.Status != "pending" {
		t.Fatalf("Status = %q, want pending", campaign.Status)
	}

	pending, err := repo.GetPendingQueries(ctx, hostA)
	if err != nil {
		t.Fatalf("GetPendingQueries(hostA): %v", err)
	}
	if got := pending[campaignID.String()]; got != "select 1" {
		t.Fatalf("pending query = %q, want %q", got, "select 1")
	}

	campaign, err = repo.GetCampaignByIDAndOrganization(ctx, campaignID, orgID)
	if err != nil {
		t.Fatalf("GetCampaignByIDAndOrganization: %v", err)
	}
	if campaign.Status != "running" {
		t.Fatalf("Status = %q, want running", campaign.Status)
	}

	res := json.RawMessage(`[{"a":"b"}]`)
	if err := repo.SaveQueryResults(ctx, hostA, campaignID, "completed", res, nil); err != nil {
		t.Fatalf("SaveQueryResults(hostA): %v", err)
	}

	campaign, err = repo.GetCampaignByIDAndOrganization(ctx, campaignID, orgID)
	if err != nil {
		t.Fatalf("GetCampaignByIDAndOrganization: %v", err)
	}
	if campaign.ResultCount != 1 {
		t.Fatalf("ResultCount = %d, want 1", campaign.ResultCount)
	}
	if campaign.Status != "running" {
		t.Fatalf("Status = %q, want running", campaign.Status)
	}

	if err := repo.SaveQueryResults(ctx, hostB, campaignID, "completed", json.RawMessage(`[]`), nil); err != nil {
		t.Fatalf("SaveQueryResults(hostB): %v", err)
	}

	campaign, err = repo.GetCampaignByIDAndOrganization(ctx, campaignID, orgID)
	if err != nil {
		t.Fatalf("GetCampaignByIDAndOrganization: %v", err)
	}
	if campaign.ResultCount != 2 {
		t.Fatalf("ResultCount = %d, want 2", campaign.ResultCount)
	}
	if campaign.Status != "completed" {
		t.Fatalf("Status = %q, want completed", campaign.Status)
	}

	campaigns, err := repo.ListCampaignsByOrganization(ctx, orgID, 10)
	if err != nil {
		t.Fatalf("ListCampaignsByOrganization: %v", err)
	}
	if len(campaigns) != 1 {
		t.Fatalf("campaigns = %d, want 1", len(campaigns))
	}
	if campaigns[0].ID != campaignID {
		t.Fatalf("campaign[0].ID = %s, want %s", campaigns[0].ID, campaignID)
	}

	targets, err := repo.GetCampaignTargets(ctx, campaignID)
	if err != nil {
		t.Fatalf("GetCampaignTargets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(targets))
	}
}
