package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CredentialRepository handles data access for WebAuthn credentials.
type CredentialRepository struct {
	pool *pgxpool.Pool
}

// ErrCredentialNotFound is returned when a credential cannot be found.
var ErrCredentialNotFound = errors.New("credential not found")

// NewCredentialRepository creates a new CredentialRepository.
func NewCredentialRepository(pool *pgxpool.Pool) *CredentialRepository {
	return &CredentialRepository{pool: pool}
}

// Create stores a new WebAuthn credential for a user.
func (r *CredentialRepository) Create(ctx context.Context, userID int, cred webauthn.Credential) error {
	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_credentials (
			user_id, credential_id, public_key, attestation_type, transports,
			flag_user_present, flag_user_verified, flag_backup_eligible, flag_backup_state,
			aaguid, sign_count, clone_warning
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		userID,
		cred.ID,
		cred.PublicKey,
		cred.AttestationType,
		transports,
		cred.Flags.UserPresent,
		cred.Flags.UserVerified,
		cred.Flags.BackupEligible,
		cred.Flags.BackupState,
		cred.Authenticator.AAGUID,
		cred.Authenticator.SignCount,
		cred.Authenticator.CloneWarning,
	)
	if err != nil {
		return fmt.Errorf("creating credential: %w", err)
	}
	return nil
}

// GetByUserID retrieves all credentials for a user.
func (r *CredentialRepository) GetByUserID(ctx context.Context, userID int) ([]webauthn.Credential, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT credential_id, public_key, attestation_type, transports,
			flag_user_present, flag_user_verified, flag_backup_eligible, flag_backup_state,
			aaguid, sign_count, clone_warning
		FROM user_credentials
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying credentials: %w", err)
	}
	defer rows.Close()

	var credentials []webauthn.Credential
	for rows.Next() {
		var (
			credID             []byte
			publicKey          []byte
			attestationType    string
			transports         []string
			flagUserPresent    bool
			flagUserVerified   bool
			flagBackupEligible bool
			flagBackupState    bool
			aaguid             []byte
			signCount          uint32
			cloneWarning       bool
		)

		if scanErr := rows.Scan(
			&credID, &publicKey, &attestationType, &transports,
			&flagUserPresent, &flagUserVerified, &flagBackupEligible, &flagBackupState,
			&aaguid, &signCount, &cloneWarning,
		); scanErr != nil {
			return nil, fmt.Errorf("scanning credential: %w", scanErr)
		}

		cred := webauthn.Credential{
			ID:              credID,
			PublicKey:       publicKey,
			AttestationType: attestationType,
			Flags: webauthn.CredentialFlags{
				UserPresent:    flagUserPresent,
				UserVerified:   flagUserVerified,
				BackupEligible: flagBackupEligible,
				BackupState:    flagBackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:       aaguid,
				SignCount:    signCount,
				CloneWarning: cloneWarning,
			},
		}

		// Convert string transports back to AuthenticatorTransport
		cred.Transport = make([]protocol.AuthenticatorTransport, len(transports))
		for i, t := range transports {
			cred.Transport[i] = protocol.AuthenticatorTransport(t)
		}

		credentials = append(credentials, cred)
	}

	if iterateErr := rows.Err(); iterateErr != nil {
		return nil, fmt.Errorf("iterating credentials: %w", iterateErr)
	}

	return credentials, nil
}

// GetByCredentialID retrieves a credential by its ID and returns the associated user.
// This is used during authentication to find which user owns a credential.
func (r *CredentialRepository) GetByCredentialID(ctx context.Context, credentialID []byte) (*webauthn.Credential, *User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT 
			uc.credential_id, uc.public_key, uc.attestation_type, uc.transports,
			uc.flag_user_present, uc.flag_user_verified, uc.flag_backup_eligible, uc.flag_backup_state,
			uc.aaguid, uc.sign_count, uc.clone_warning,
			u.id, u.email, u.password_hash
		FROM user_credentials uc
		JOIN users u ON u.id = uc.user_id
		WHERE uc.credential_id = $1
	`, credentialID)

	var (
		credID             []byte
		publicKey          []byte
		attestationType    string
		transports         []string
		flagUserPresent    bool
		flagUserVerified   bool
		flagBackupEligible bool
		flagBackupState    bool
		aaguid             []byte
		signCount          uint32
		cloneWarning       bool
		userID             int
		userEmail          string
		userPasswordHash   string
	)

	if err := row.Scan(
		&credID, &publicKey, &attestationType, &transports,
		&flagUserPresent, &flagUserVerified, &flagBackupEligible, &flagBackupState,
		&aaguid, &signCount, &cloneWarning,
		&userID, &userEmail, &userPasswordHash,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrCredentialNotFound
		}
		return nil, nil, fmt.Errorf("querying credential by id: %w", err)
	}

	cred := &webauthn.Credential{
		ID:              credID,
		PublicKey:       publicKey,
		AttestationType: attestationType,
		Flags: webauthn.CredentialFlags{
			UserPresent:    flagUserPresent,
			UserVerified:   flagUserVerified,
			BackupEligible: flagBackupEligible,
			BackupState:    flagBackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       aaguid,
			SignCount:    signCount,
			CloneWarning: cloneWarning,
		},
	}

	cred.Transport = make([]protocol.AuthenticatorTransport, len(transports))
	for i, t := range transports {
		cred.Transport[i] = protocol.AuthenticatorTransport(t)
	}

	user := &User{
		ID:           userID,
		Email:        userEmail,
		PasswordHash: userPasswordHash,
	}

	return cred, user, nil
}

// UpdateSignCount updates the sign count after a successful authentication.
// This helps detect cloned authenticators.
func (r *CredentialRepository) UpdateSignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_credentials
		SET sign_count = $1, last_used_at = $2
		WHERE credential_id = $3
	`, signCount, time.Now(), credentialID)
	if err != nil {
		return fmt.Errorf("updating sign count: %w", err)
	}
	return nil
}

// Delete removes a credential by its ID.
func (r *CredentialRepository) Delete(ctx context.Context, credentialID []byte) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM user_credentials WHERE credential_id = $1
	`, credentialID)
	if err != nil {
		return fmt.Errorf("deleting credential: %w", err)
	}
	return nil
}

// CountByUserID returns the number of credentials a user has.
func (r *CredentialRepository) CountByUserID(ctx context.Context, userID int) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM user_credentials WHERE user_id = $1
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting credentials: %w", err)
	}
	return count, nil
}

// PasskeyInfo represents a passkey for display in the UI.
type PasskeyInfo struct {
	ID         string     // Base64-encoded credential_id for use in URLs/forms
	Nickname   string     // User-provided name (e.g., "MacBook Pro")
	CreatedAt  time.Time  // When the passkey was registered
	LastUsedAt *time.Time // When the passkey was last used (nil if never)
}

// GetPasskeysByUserID retrieves passkey display info for a user.
func (r *CredentialRepository) GetPasskeysByUserID(ctx context.Context, userID int) ([]PasskeyInfo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT credential_id, nickname, created_at, last_used_at
		FROM user_credentials
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying passkeys: %w", err)
	}
	defer rows.Close()

	var passkeys []PasskeyInfo
	for rows.Next() {
		var (
			credID     []byte
			nickname   *string
			createdAt  time.Time
			lastUsedAt *time.Time
		)

		if scanErr := rows.Scan(&credID, &nickname, &createdAt, &lastUsedAt); scanErr != nil {
			return nil, fmt.Errorf("scanning passkey: %w", scanErr)
		}

		info := PasskeyInfo{
			ID:         base64.RawURLEncoding.EncodeToString(credID),
			CreatedAt:  createdAt,
			LastUsedAt: lastUsedAt,
		}
		if nickname != nil {
			info.Nickname = *nickname
		}

		passkeys = append(passkeys, info)
	}

	if iterateErr := rows.Err(); iterateErr != nil {
		return nil, fmt.Errorf("iterating passkeys: %w", iterateErr)
	}

	return passkeys, nil
}

// UpdateNickname updates the nickname for a credential.
func (r *CredentialRepository) UpdateNickname(ctx context.Context, credentialID []byte, nickname string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_credentials
		SET nickname = $1
		WHERE credential_id = $2
	`, nickname, credentialID)
	if err != nil {
		return fmt.Errorf("updating nickname: %w", err)
	}
	return nil
}

// CreateWithNickname stores a new WebAuthn credential with an optional nickname.
func (r *CredentialRepository) CreateWithNickname(ctx context.Context, userID int, cred webauthn.Credential, nickname string) error {
	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	var nicknamePtr *string
	if nickname != "" {
		nicknamePtr = &nickname
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_credentials (
			user_id, credential_id, public_key, attestation_type, transports,
			flag_user_present, flag_user_verified, flag_backup_eligible, flag_backup_state,
			aaguid, sign_count, clone_warning, nickname
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		userID,
		cred.ID,
		cred.PublicKey,
		cred.AttestationType,
		transports,
		cred.Flags.UserPresent,
		cred.Flags.UserVerified,
		cred.Flags.BackupEligible,
		cred.Flags.BackupState,
		cred.Authenticator.AAGUID,
		cred.Authenticator.SignCount,
		cred.Authenticator.CloneWarning,
		nicknamePtr,
	)
	if err != nil {
		return fmt.Errorf("creating credential with nickname: %w", err)
	}
	return nil
}

// DeleteByUserAndID deletes a credential by its ID, but only if it belongs to the user.
// Returns true if a row was deleted, false if no matching row was found.
func (r *CredentialRepository) DeleteByUserAndID(ctx context.Context, userID int, credentialID []byte) (bool, error) {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM user_credentials
		WHERE user_id = $1 AND credential_id = $2
	`, userID, credentialID)
	if err != nil {
		return false, fmt.Errorf("deleting credential: %w", err)
	}
	return result.RowsAffected() > 0, nil
}
