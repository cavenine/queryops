package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrganizationMember struct {
	UserID         int       `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
}

type OrganizationEnrollSecret struct {
	Secret         string    `json:"secret"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
}

var ErrOrganizationNotFound = errors.New("organization not found")

type OrganizationRepository struct {
	pool *pgxpool.Pool
}

func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{pool: pool}
}

func (r *OrganizationRepository) Create(ctx context.Context, name string, ownerID int) (*Organization, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	org := &Organization{}
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
	`, name).Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, errors.New("organization name already exists")
		}
		return nil, fmt.Errorf("inserting organization: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO organization_members (user_id, organization_id, role)
		VALUES ($1, $2, 'owner')
	`, ownerID, org.ID)

	if err != nil {
		return nil, fmt.Errorf("adding owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return org, nil
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org := &Organization{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`, id).Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("querying organization by id: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepository) GetUserOrganizations(ctx context.Context, userID int) ([]*Organization, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.name, o.created_at, o.updated_at
		FROM organizations o
		JOIN organization_members om ON o.id = om.organization_id
		WHERE om.user_id = $1
		ORDER BY o.created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user organizations: %w", err)
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		org := &Organization{}
		if err := rows.Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning organization: %w", err)
		}
		orgs = append(orgs, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("querying user organizations: %w", err)
	}
	return orgs, nil
}

func (r *OrganizationRepository) AddEnrollSecret(ctx context.Context, organizationID uuid.UUID, secret string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE organization_enroll_secrets
		SET active = false
		WHERE organization_id = $1 AND active = true
	`, organizationID)
	if err != nil {
		return fmt.Errorf("deactivating enroll secrets: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO organization_enroll_secrets (secret, organization_id, active)
		VALUES ($1, $2, true)
	`, secret, organizationID)
	if err != nil {
		return fmt.Errorf("inserting enroll secret: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing enroll secret: %w", err)
	}

	return nil
}

func (r *OrganizationRepository) GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*Organization, error) {
	org := &Organization{}
	err := r.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.created_at, o.updated_at
		FROM organizations o
		JOIN organization_enroll_secrets oes ON o.id = oes.organization_id
		WHERE oes.secret = $1 AND oes.active = true
	`, secret).Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("querying organization by secret: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepository) GetActiveEnrollSecret(ctx context.Context, organizationID uuid.UUID) (*OrganizationEnrollSecret, error) {
	secret := &OrganizationEnrollSecret{}
	err := r.pool.QueryRow(ctx, `
		SELECT secret, organization_id, active, created_at
		FROM organization_enroll_secrets
		WHERE organization_id = $1 AND active = true
		ORDER BY created_at DESC
		LIMIT 1
	`, organizationID).Scan(&secret.Secret, &secret.OrganizationID, &secret.Active, &secret.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No active secret
		}
		return nil, fmt.Errorf("querying active secret: %w", err)
	}
	return secret, nil
}
