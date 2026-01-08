package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cavenine/queryops/features/auth/services"
	"github.com/cavenine/queryops/internal/testdb"
)

func TestUserRepository_GetByEmail(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	if _, err := tdb.Pool.Exec(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2)`, "test@example.com", "hash"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewUserRepository(tdb.Pool)

	tests := []struct {
		name    string
		email   string
		wantErr bool
		err     error
	}{
		{
			name:    "found",
			email:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "not found",
			email:   "notfound@example.com",
			wantErr: true,
			err:     services.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetByEmail(ctx, tt.email)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}
				return
			}
			if user.Email != tt.email {
				t.Fatalf("Email = %q, want %q", user.Email, tt.email)
			}
		})
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewUserRepository(tdb.Pool)

	tests := []struct {
		name    string
		userID  int
		wantErr bool
		err     error
	}{
		{
			name:    "found",
			userID:  userID,
			wantErr: false,
		},
		{
			name:    "not found",
			userID:  99999,
			wantErr: true,
			err:     services.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetByID(ctx, tt.userID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}
				return
			}
			if user.ID != tt.userID {
				t.Fatalf("ID = %d, want %d", user.ID, tt.userID)
			}
		})
	}
}

func TestUserRepository_Create(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	repo := services.NewUserRepository(tdb.Pool)

	tests := []struct {
		name         string
		email        string
		passwordHash string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "success",
			email:        "new@example.com",
			passwordHash: "hashedpassword",
			wantErr:      false,
		},
		{
			name:         "duplicate email",
			email:        "duplicate@example.com",
			passwordHash: "hash",
			wantErr:      true,
			errContains:  "email already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "duplicate email" {
				if _, err := tdb.Pool.Exec(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2)`, tt.email, tt.passwordHash); err != nil {
					t.Fatalf("seed user: %v", err)
				}
			}

			user, err := repo.Create(ctx, tt.email, tt.passwordHash)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error = %q, want contains %q", err.Error(), tt.errContains)
				}
				return
			}
			if user.Email != tt.email {
				t.Fatalf("Email = %q, want %q", user.Email, tt.email)
			}
		})
	}
}

func TestUserRepository_EmailExists(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	if _, err := tdb.Pool.Exec(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2)`, "exists@example.com", "hash"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	repo := services.NewUserRepository(tdb.Pool)

	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{
			name:  "exists",
			email: "exists@example.com",
			want:  true,
		},
		{
			name:  "not exists",
			email: "notexists@example.com",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.EmailExists(ctx, tt.email)
			if err != nil {
				t.Fatalf("EmailExists() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("EmailExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
