package services

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents a user account in the system.
type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"` // Never expose hash

	// Credentials holds the user's WebAuthn credentials (passkeys).
	// Populated by loading from user_credentials table when needed.
	Credentials []webauthn.Credential `json:"-"`
}

// ErrUserNotFound is returned when a user cannot be found.
var ErrUserNotFound = errors.New("user not found")

// WebAuthnID returns a unique identifier for the user (required by webauthn.User interface).
// We use the user's database ID encoded as bytes.
func (u *User) WebAuthnID() []byte {
	const idBytes = 8
	buf := make([]byte, idBytes)
	id := max(0, u.ID)
	// #nosec G115
	binary.BigEndian.PutUint64(buf, uint64(id))
	return buf
}

// WebAuthnName returns the user's identifier (email) for display during WebAuthn ceremonies.
func (u *User) WebAuthnName() string {
	return u.Email
}

// WebAuthnDisplayName returns a human-friendly name for the user.
func (u *User) WebAuthnDisplayName() string {
	return u.Email
}

// WebAuthnCredentials returns the user's registered WebAuthn credentials.
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// HasPassword returns true if the user has a password set.
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}

// UserRepository handles data access for users.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// GetByEmail retrieves a user by their email address.
// Returns ErrUserNotFound if no user found.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash
		FROM users
		WHERE email = $1
	`, email)

	user := &User{}
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("querying user by email: %w", err)
	}

	return user, nil
}

// GetByID retrieves a user by their ID.
// Returns ErrUserNotFound if no user found.
func (r *UserRepository) GetByID(ctx context.Context, id int) (*User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash
		FROM users
		WHERE id = $1
	`, id)

	user := &User{}
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("querying user by id: %w", err)
	}

	return user, nil
}

// Create inserts a new user into the database.
func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (*User, error) {
	user := &User{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash
	`, email, passwordHash).Scan(&user.ID, &user.Email, &user.PasswordHash)

	if err != nil {
		// Check for unique violation (PostgreSQL error code 23505)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, errors.New("email already registered")
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return user, nil
}

// EmailExists checks if an email is already registered.
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`, email).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("checking email existence: %w", err)
	}

	return exists, nil
}
