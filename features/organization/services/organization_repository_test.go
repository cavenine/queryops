package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	orgservices "github.com/cavenine/queryops/features/organization/services"
	"github.com/cavenine/queryops/internal/testdb"
	"github.com/google/uuid"
)

func TestOrganizationRepository_Create(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var ownerID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "owner@example.com", "hash").Scan(&ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}

	repo := orgservices.NewOrganizationRepository(tdb.Pool)

	tests := []struct {
		name        string
		orgName     string
		ownerID     int
		wantErr     bool
		errContains string
	}{
		{
			name:    "success",
			orgName: "Test Org",
			ownerID: ownerID,
			wantErr: false,
		},
		{
			name:        "duplicate name",
			orgName:     "Duplicate Org",
			ownerID:     ownerID,
			wantErr:     true,
			errContains: "organization name already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "duplicate name" {
				if _, err := repo.Create(ctx, tt.orgName, tt.ownerID); err != nil {
					t.Fatalf("seed org: %v", err)
				}
			}

			org, err := repo.Create(ctx, tt.orgName, tt.ownerID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error = %q, want contains %q", err.Error(), tt.errContains)
				}
				return
			}
			if org.Name != tt.orgName {
				t.Fatalf("Name = %q, want %q", org.Name, tt.orgName)
			}

			var memberCount int
			if err := tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM organization_members WHERE organization_id = $1 AND user_id = $2`, org.ID, tt.ownerID).Scan(&memberCount); err != nil {
				t.Fatalf("count members: %v", err)
			}
			if memberCount != 1 {
				t.Fatalf("expected 1 owner member, got %d", memberCount)
			}
		})
	}
}

func TestOrganizationRepository_GetByID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "user@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := orgservices.NewOrganizationRepository(tdb.Pool)
	org, err := repo.Create(ctx, "Test Org", userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tests := []struct {
		name    string
		orgID   uuid.UUID
		wantErr bool
		err     error
	}{
		{
			name:    "found",
			orgID:   org.ID,
			wantErr: false,
		},
		{
			name:    "not found",
			orgID:   uuid.New(),
			wantErr: true,
			err:     orgservices.ErrOrganizationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetByID(ctx, tt.orgID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}
				return
			}
			if got.ID != tt.orgID {
				t.Fatalf("ID = %s, want %s", got.ID, tt.orgID)
			}
		})
	}
}

func TestOrganizationRepository_GetUserOrganizations(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID1, userID2 int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "user1@example.com", "hash").Scan(&userID1); err != nil {
		t.Fatalf("insert user1: %v", err)
	}
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "user2@example.com", "hash").Scan(&userID2); err != nil {
		t.Fatalf("insert user2: %v", err)
	}

	repo := orgservices.NewOrganizationRepository(tdb.Pool)

	if _, err := repo.Create(ctx, "Org 1", userID1); err != nil {
		t.Fatalf("Create(Org 1) error = %v", err)
	}
	if _, err := repo.Create(ctx, "Org 2", userID1); err != nil {
		t.Fatalf("Create(Org 2) error = %v", err)
	}
	org3, err := repo.Create(ctx, "Org 3", userID2)
	if err != nil {
		t.Fatalf("Create(Org 3) error = %v", err)
	}

	if _, err := tdb.Pool.Exec(ctx, `INSERT INTO organization_members (user_id, organization_id, role) VALUES ($1, $2, 'member')`, userID1, org3.ID); err != nil {
		t.Fatalf("seed member: %v", err)
	}

	tests := []struct {
		name    string
		userID  int
		wantLen int
	}{
		{
			name:    "user with multiple orgs",
			userID:  userID1,
			wantLen: 3,
		},
		{
			name:    "user with one org",
			userID:  userID2,
			wantLen: 1,
		},
		{
			name:    "user with no orgs",
			userID:  99999,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgs, err := repo.GetUserOrganizations(ctx, tt.userID)
			if err != nil {
				t.Fatalf("GetUserOrganizations() error = %v", err)
			}
			if len(orgs) != tt.wantLen {
				t.Fatalf("len(orgs) = %d, want %d", len(orgs), tt.wantLen)
			}
		})
	}
}

func TestOrganizationRepository_AddEnrollSecret(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "user@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := orgservices.NewOrganizationRepository(tdb.Pool)
	org, err := repo.Create(ctx, "Test Org", userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	secret1 := "secret-001"
	if err := repo.AddEnrollSecret(ctx, org.ID, secret1); err != nil {
		t.Fatalf("AddEnrollSecret(first) error = %v", err)
	}

	var activeSecret string
	if err := tdb.Pool.QueryRow(ctx, `SELECT secret FROM organization_enroll_secrets WHERE organization_id = $1 AND active = true`, org.ID).Scan(&activeSecret); err != nil {
		t.Fatalf("select active secret: %v", err)
	}
	if activeSecret != secret1 {
		t.Fatalf("active secret = %q, want %q", activeSecret, secret1)
	}

	secret2 := "secret-002"
	if err := repo.AddEnrollSecret(ctx, org.ID, secret2); err != nil {
		t.Fatalf("AddEnrollSecret(second) error = %v", err)
	}

	if err := tdb.Pool.QueryRow(ctx, `SELECT secret FROM organization_enroll_secrets WHERE organization_id = $1 AND active = true`, org.ID).Scan(&activeSecret); err != nil {
		t.Fatalf("select active secret after rotation: %v", err)
	}
	if activeSecret != secret2 {
		t.Fatalf("active secret after rotation = %q, want %q", activeSecret, secret2)
	}

	var inactiveCount int
	if err := tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM organization_enroll_secrets WHERE organization_id = $1 AND active = false`, org.ID).Scan(&inactiveCount); err != nil {
		t.Fatalf("select inactive count: %v", err)
	}
	if inactiveCount != 1 {
		t.Fatalf("inactive secrets = %d, want 1", inactiveCount)
	}
}

func TestOrganizationRepository_GetOrganizationByEnrollSecret(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "user@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := orgservices.NewOrganizationRepository(tdb.Pool)
	org, err := repo.Create(ctx, "Test Org", userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	inactiveSecret := "inactive-secret"
	activeSecret := "active-secret"
	if err := repo.AddEnrollSecret(ctx, org.ID, inactiveSecret); err != nil {
		t.Fatalf("AddEnrollSecret(first) error = %v", err)
	}
	if err := repo.AddEnrollSecret(ctx, org.ID, activeSecret); err != nil {
		t.Fatalf("AddEnrollSecret(second) error = %v", err)
	}

	tests := []struct {
		name    string
		secret  string
		wantErr bool
		err     error
	}{
		{
			name:    "active secret",
			secret:  activeSecret,
			wantErr: false,
		},
		{
			name:    "inactive secret",
			secret:  inactiveSecret,
			wantErr: true,
			err:     orgservices.ErrOrganizationNotFound,
		},
		{
			name:    "unknown secret",
			secret:  "unknown",
			wantErr: true,
			err:     orgservices.ErrOrganizationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetOrganizationByEnrollSecret(ctx, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetOrganizationByEnrollSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}
				return
			}
			if got.ID != org.ID {
				t.Fatalf("ID = %s, want %s", got.ID, org.ID)
			}
		})
	}
}

func TestOrganizationRepository_GetActiveEnrollSecret(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "user@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := orgservices.NewOrganizationRepository(tdb.Pool)
	org, err := repo.Create(ctx, "Test Org", userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	secret, err := repo.GetActiveEnrollSecret(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetActiveEnrollSecret(no secrets) error = %v", err)
	}
	if secret != nil {
		t.Fatal("expected nil, got secret")
	}

	secretValue := "my-enroll-secret"
	if err := repo.AddEnrollSecret(ctx, org.ID, secretValue); err != nil {
		t.Fatalf("AddEnrollSecret() error = %v", err)
	}

	secret, err = repo.GetActiveEnrollSecret(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetActiveEnrollSecret() error = %v", err)
	}
	if secret == nil {
		t.Fatal("expected secret, got nil")
	}
	if secret.Secret != secretValue {
		t.Fatalf("Secret = %q, want %q", secret.Secret, secretValue)
	}
	if !secret.Active {
		t.Fatal("expected Active = true")
	}
	if secret.OrganizationID != org.ID {
		t.Fatalf("OrganizationID = %s, want %s", secret.OrganizationID, org.ID)
	}
}
