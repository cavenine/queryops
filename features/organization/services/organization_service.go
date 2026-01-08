package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type organizationRepository interface {
	Create(ctx context.Context, name string, ownerID int) (*Organization, error)
	AddEnrollSecret(ctx context.Context, orgID uuid.UUID, secret string) error
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetUserOrganizations(ctx context.Context, userID int) ([]*Organization, error)
	GetActiveEnrollSecret(ctx context.Context, orgID uuid.UUID) (*OrganizationEnrollSecret, error)
	GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*Organization, error)
}

type OrganizationService struct {
	repo organizationRepository
}

func NewOrganizationService(repo organizationRepository) *OrganizationService {
	return &OrganizationService{repo: repo}
}

func (s *OrganizationService) Create(ctx context.Context, name string, ownerID int) (*Organization, error) {
	// Create organization and assign owner
	org, err := s.repo.Create(ctx, name, ownerID)
	if err != nil {
		return nil, err
	}

	// Generate and assign enrollment secret
	secret, err := s.GenerateEnrollSecret(name)
	if err != nil {
		return nil, fmt.Errorf("generating secret: %w", err)
	}

	if err := s.repo.AddEnrollSecret(ctx, org.ID, secret); err != nil {
		return nil, fmt.Errorf("adding secret: %w", err)
	}

	return org, nil
}

func (s *OrganizationService) GenerateEnrollSecret(orgName string) (string, error) {
	// Normalize org name for secret (lowercase, no spaces)
	prefix := strings.ToLower(strings.ReplaceAll(orgName, " ", ""))
	// remove non-alphanumeric characters
	prefix = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, prefix)

	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	suffix := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s-%s", prefix, suffix), nil
}

func (s *OrganizationService) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *OrganizationService) GetUserOrganizations(ctx context.Context, userID int) ([]*Organization, error) {
	return s.repo.GetUserOrganizations(ctx, userID)
}

func (s *OrganizationService) GetActiveEnrollSecret(ctx context.Context, orgID uuid.UUID) (string, error) {
	secret, err := s.repo.GetActiveEnrollSecret(ctx, orgID)
	if err != nil {
		return "", err
	}
	if secret == nil {
		return "", nil
	}
	return secret.Secret, nil
}

func (s *OrganizationService) GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*Organization, error) {
	return s.repo.GetOrganizationByEnrollSecret(ctx, secret)
}
