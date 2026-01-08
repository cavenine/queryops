package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cavenine/queryops/features/auth/services"
	"github.com/cavenine/queryops/internal/testdb"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestCredentialRepository_Create(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	cred := webauthn.Credential{
		ID:              []byte("cred-id-123"),
		PublicKey:       []byte("public-key-bytes"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal, protocol.Hybrid},
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			UserVerified:   true,
			BackupEligible: true,
			BackupState:    false,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
			SignCount:    0,
			CloneWarning: false,
		},
	}

	if err := repo.Create(ctx, userID, cred); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var count int
	if err := tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_credentials WHERE user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("select count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 credential, got %d", count)
	}
}

func TestCredentialRepository_GetByUserID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	cred1 := webauthn.Credential{
		ID:              []byte("cred-id-1"),
		PublicKey:       []byte("public-key-1"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
		Flags: webauthn.CredentialFlags{
			UserPresent:  true,
			UserVerified: true,
		},
		Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
	}

	cred2 := webauthn.Credential{
		ID:              []byte("cred-id-2"),
		PublicKey:       []byte("public-key-2"),
		AttestationType: "packed",
		Transport:       []protocol.AuthenticatorTransport{protocol.Hybrid},
		Flags: webauthn.CredentialFlags{
			UserPresent:  true,
			UserVerified: false,
		},
		Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 1},
	}

	if err := repo.Create(ctx, userID, cred1); err != nil {
		t.Fatalf("Create(cred1) error = %v", err)
	}
	if err := repo.Create(ctx, userID, cred2); err != nil {
		t.Fatalf("Create(cred2) error = %v", err)
	}

	creds, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("GetByUserID() returned %d credentials, want 2", len(creds))
	}
}

func TestCredentialRepository_GetByCredentialID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	cred := webauthn.Credential{
		ID:              []byte("cred-id-lookup"),
		PublicKey:       []byte("public-key"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
		Flags: webauthn.CredentialFlags{
			UserPresent:  true,
			UserVerified: true,
		},
		Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
	}

	if err := repo.Create(ctx, userID, cred); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tests := []struct {
		name         string
		credentialID []byte
		wantCred     bool
		wantUser     bool
		wantErr      bool
		err          error
	}{
		{
			name:         "found",
			credentialID: []byte("cred-id-lookup"),
			wantCred:     true,
			wantUser:     true,
			wantErr:      false,
		},
		{
			name:         "not found",
			credentialID: []byte("nonexistent"),
			wantCred:     false,
			wantUser:     false,
			wantErr:      true,
			err:          services.ErrCredentialNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, user, err := repo.GetByCredentialID(ctx, tt.credentialID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByCredentialID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}
				return
			}
			if !tt.wantCred || cred == nil {
				t.Fatal("expected credential")
			}
			if !tt.wantUser || user == nil {
				t.Fatal("expected user")
			}
			if user.ID != userID {
				t.Fatalf("user ID = %d, want %d", user.ID, userID)
			}
		})
	}
}

func TestCredentialRepository_UpdateSignCount(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	cred := webauthn.Credential{
		ID:              []byte("cred-id-update"),
		PublicKey:       []byte("public-key"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
		Flags: webauthn.CredentialFlags{
			UserPresent:  true,
			UserVerified: true,
		},
		Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
	}

	if err := repo.Create(ctx, userID, cred); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	newSignCount := uint32(5)
	if err := repo.UpdateSignCount(ctx, []byte("cred-id-update"), newSignCount); err != nil {
		t.Fatalf("UpdateSignCount() error = %v", err)
	}

	var signCount uint32
	if err := tdb.Pool.QueryRow(ctx, `SELECT sign_count FROM user_credentials WHERE credential_id = $1`, []byte("cred-id-update")).Scan(&signCount); err != nil {
		t.Fatalf("select sign_count: %v", err)
	}
	if signCount != newSignCount {
		t.Fatalf("sign_count = %d, want %d", signCount, newSignCount)
	}

	var lastUsed *time.Time
	if err := tdb.Pool.QueryRow(ctx, `SELECT last_used_at FROM user_credentials WHERE credential_id = $1`, []byte("cred-id-update")).Scan(&lastUsed); err != nil {
		t.Fatalf("select last_used_at: %v", err)
	}
	if lastUsed == nil {
		t.Fatal("last_used_at should be set")
	}
}

func TestCredentialRepository_CountByUserID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	repo := services.NewCredentialRepository(tdb.Pool)

	tests := []struct {
		name   string
		userID int
		setup  func() int
		want   int
	}{
		{
			name:   "zero credentials",
			userID: 99999,
			setup:  func() int { return 99999 },
			want:   0,
		},
		{
			name: "three credentials",
			setup: func() int {
				var uid int
				if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "count@example.com", "hash").Scan(&uid); err != nil {
					t.Fatalf("insert user: %v", err)
				}
				for i := 0; i < 3; i++ {
					cred := webauthn.Credential{
						ID:              []byte{byte('a' + i)},
						PublicKey:       []byte("public-key"),
						AttestationType: "none",
						Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
						Flags:           webauthn.CredentialFlags{UserPresent: true, UserVerified: true},
						Authenticator:   webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
					}
					if err := repo.Create(ctx, uid, cred); err != nil {
						t.Fatalf("Create(cred) error: %v", err)
					}
				}
				return uid
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.userID = tt.setup()
			}
			got, err := repo.CountByUserID(ctx, tt.userID)
			if err != nil {
				t.Fatalf("CountByUserID() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("CountByUserID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCredentialRepository_GetPasskeysByUserID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	cred := webauthn.Credential{
		ID:              []byte("cred-id-passkey"),
		PublicKey:       []byte("public-key"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
		Flags: webauthn.CredentialFlags{
			UserPresent:  true,
			UserVerified: true,
		},
		Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
	}

	if err := repo.CreateWithNickname(ctx, userID, cred, "My Passkey"); err != nil {
		t.Fatalf("CreateWithNickname() error = %v", err)
	}

	passkeys, err := repo.GetPasskeysByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetPasskeysByUserID() error = %v", err)
	}
	if len(passkeys) != 1 {
		t.Fatalf("GetPasskeysByUserID() returned %d passkeys, want 1", len(passkeys))
	}
	if passkeys[0].Nickname != "My Passkey" {
		t.Fatalf("Nickname = %q, want %q", passkeys[0].Nickname, "My Passkey")
	}
}

func TestCredentialRepository_UpdateNickname(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	cred := webauthn.Credential{
		ID:              []byte("cred-id-nick"),
		PublicKey:       []byte("public-key"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
		Flags:           webauthn.CredentialFlags{UserPresent: true, UserVerified: true},
		Authenticator:   webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
	}

	if err := repo.CreateWithNickname(ctx, userID, cred, "Old Nickname"); err != nil {
		t.Fatalf("CreateWithNickname() error = %v", err)
	}

	if err := repo.UpdateNickname(ctx, []byte("cred-id-nick"), "New Nickname"); err != nil {
		t.Fatalf("UpdateNickname() error = %v", err)
	}

	var nickname string
	if err := tdb.Pool.QueryRow(ctx, `SELECT nickname FROM user_credentials WHERE credential_id = $1`, []byte("cred-id-nick")).Scan(&nickname); err != nil {
		t.Fatalf("select nickname: %v", err)
	}
	if nickname != "New Nickname" {
		t.Fatalf("nickname = %q, want %q", nickname, "New Nickname")
	}
}

func TestCredentialRepository_DeleteByUserAndID(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	var userID int
	if err := tdb.Pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, "test@example.com", "hash").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := services.NewCredentialRepository(tdb.Pool)

	tests := []struct {
		name               string
		deleteUserID       int
		deleteCredentialID []byte
		seedCredentialID   []byte
		wantDeleted        bool
		wantRemaining      int
	}{
		{
			name:               "delete matching",
			deleteUserID:       userID,
			deleteCredentialID: []byte("cred-id-delete-1"),
			seedCredentialID:   []byte("cred-id-delete-1"),
			wantDeleted:        true,
			wantRemaining:      0,
		},
		{
			name:               "delete non-matching user",
			deleteUserID:       99999,
			deleteCredentialID: []byte("cred-id-delete-2"),
			seedCredentialID:   []byte("cred-id-delete-2"),
			wantDeleted:        false,
			wantRemaining:      1,
		},
		{
			name:               "delete non-matching cred",
			deleteUserID:       userID,
			deleteCredentialID: []byte("nonexistent"),
			seedCredentialID:   []byte("cred-id-delete-3"),
			wantDeleted:        false,
			wantRemaining:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := webauthn.Credential{
				ID:              tt.seedCredentialID,
				PublicKey:       []byte("public-key"),
				AttestationType: "none",
				Transport:       []protocol.AuthenticatorTransport{protocol.Internal},
				Flags:           webauthn.CredentialFlags{UserPresent: true, UserVerified: true},
				Authenticator:   webauthn.Authenticator{AAGUID: make([]byte, 16), SignCount: 0},
			}

			if err := repo.Create(ctx, userID, cred); err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			deleted, err := repo.DeleteByUserAndID(ctx, tt.deleteUserID, tt.deleteCredentialID)
			if err != nil {
				t.Fatalf("DeleteByUserAndID() error = %v", err)
			}
			if deleted != tt.wantDeleted {
				t.Fatalf("deleted = %v, want %v", deleted, tt.wantDeleted)
			}

			var count int
			if err := tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_credentials WHERE credential_id = $1`, tt.seedCredentialID).Scan(&count); err != nil {
				t.Fatalf("select count: %v", err)
			}
			if count != tt.wantRemaining {
				t.Fatalf("remaining = %d, want %d", count, tt.wantRemaining)
			}
		})
	}
}
