package services_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/cavenine/queryops/features/organization/services"
	"github.com/google/uuid"
)

type stubOrgRepo struct {
	createFunc                func(ctx context.Context, name string, ownerID int) (*services.Organization, error)
	addEnrollSecretFunc       func(ctx context.Context, orgID uuid.UUID, secret string) error
	getByIDFunc               func(ctx context.Context, id uuid.UUID) (*services.Organization, error)
	getUserOrganizationsFunc  func(ctx context.Context, userID int) ([]*services.Organization, error)
	getActiveEnrollSecretFunc func(ctx context.Context, orgID uuid.UUID) (*services.OrganizationEnrollSecret, error)
	getOrgByEnrollSecretFunc  func(ctx context.Context, secret string) (*services.Organization, error)
}

func (s *stubOrgRepo) Create(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, name, ownerID)
	}
	return nil, nil
}

func (s *stubOrgRepo) AddEnrollSecret(ctx context.Context, orgID uuid.UUID, secret string) error {
	if s.addEnrollSecretFunc != nil {
		return s.addEnrollSecretFunc(ctx, orgID, secret)
	}
	return nil
}

func (s *stubOrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*services.Organization, error) {
	if s.getByIDFunc != nil {
		return s.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (s *stubOrgRepo) GetUserOrganizations(ctx context.Context, userID int) ([]*services.Organization, error) {
	if s.getUserOrganizationsFunc != nil {
		return s.getUserOrganizationsFunc(ctx, userID)
	}
	return nil, nil
}

func (s *stubOrgRepo) GetActiveEnrollSecret(ctx context.Context, orgID uuid.UUID) (*services.OrganizationEnrollSecret, error) {
	if s.getActiveEnrollSecretFunc != nil {
		return s.getActiveEnrollSecretFunc(ctx, orgID)
	}
	return nil, nil
}

func (s *stubOrgRepo) GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*services.Organization, error) {
	if s.getOrgByEnrollSecretFunc != nil {
		return s.getOrgByEnrollSecretFunc(ctx, secret)
	}
	return nil, nil
}

func TestCreate_Success(t *testing.T) {
	orgID := uuid.New()

	repo := &stubOrgRepo{
		createFunc: func(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
			if name != "MyOrg" {
				t.Errorf("expected name MyOrg, got %s", name)
			}
			if ownerID != 42 {
				t.Errorf("expected ownerID 42, got %d", ownerID)
			}
			return &services.Organization{ID: orgID, Name: name}, nil
		},
		addEnrollSecretFunc: func(ctx context.Context, id uuid.UUID, secret string) error {
			if id != orgID {
				t.Errorf("expected orgID %s, got %s", orgID, id)
			}
			matched, _ := regexp.MatchString(`^myorg-[0-9a-f]{16}$`, secret)
			if !matched {
				t.Errorf("expected secret format myorg-<hex16>, got %s", secret)
			}
			return nil
		},
	}
	service := services.NewOrganizationService(repo)

	org, err := service.Create(context.Background(), "MyOrg", 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org == nil {
		t.Fatal("expected org, got nil")
	}
	if org.ID != orgID {
		t.Errorf("expected ID %s, got %s", orgID, org.ID)
	}
}

func TestCreate_RepoCreateError(t *testing.T) {
	repo := &stubOrgRepo{
		createFunc: func(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
			return nil, services.ErrOrganizationNotFound
		},
	}
	service := services.NewOrganizationService(repo)

	_, err := service.Create(context.Background(), "TestOrg", 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, services.ErrOrganizationNotFound) {
		t.Errorf("expected ErrOrganizationNotFound, got: %v", err)
	}
}

func TestCreate_EnrollSecretError(t *testing.T) {
	orgID := uuid.New()

	repo := &stubOrgRepo{
		createFunc: func(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
			return &services.Organization{ID: orgID, Name: name}, nil
		},
		addEnrollSecretFunc: func(ctx context.Context, id uuid.UUID, secret string) error {
			return errors.New("db error")
		},
	}
	service := services.NewOrganizationService(repo)

	_, err := service.Create(context.Background(), "TestOrg", 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err == nil || err.Error() != "adding secret: db error" {
		t.Errorf("expected 'adding secret: db error', got: %v", err)
	}
}

func TestGenerateEnrollSecret_SimpleName(t *testing.T) {
	service := services.NewOrganizationService(nil)

	secret, err := service.GenerateEnrollSecret("MyOrg")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	matched, _ := regexp.MatchString(`^myorg-[0-9a-f]{16}$`, secret)
	if !matched {
		t.Errorf("expected format myorg-<hex16>, got %s", secret)
	}
}

func TestGenerateEnrollSecret_NameWithSpaces(t *testing.T) {
	service := services.NewOrganizationService(nil)

	secret, err := service.GenerateEnrollSecret("My Test Org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	matched, _ := regexp.MatchString(`^mytestorg-[0-9a-f]{16}$`, secret)
	if !matched {
		t.Errorf("expected format mytestorg-<hex16>, got %s", secret)
	}
}

func TestGenerateEnrollSecret_NameWithSpecialChars(t *testing.T) {
	service := services.NewOrganizationService(nil)

	secret, err := service.GenerateEnrollSecret("Org@123!")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	matched, _ := regexp.MatchString(`^org123-[0-9a-f]{16}$`, secret)
	if !matched {
		t.Errorf("expected format org123-<hex16>, got %s", secret)
	}
}

func TestGetByID_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &stubOrgRepo{
		getByIDFunc: func(ctx context.Context, id uuid.UUID) (*services.Organization, error) {
			if id != orgID {
				t.Errorf("expected ID %s, got %s", orgID, id)
			}
			return &services.Organization{ID: id, Name: "TestOrg"}, nil
		},
	}
	service := services.NewOrganizationService(repo)

	org, err := service.GetByID(context.Background(), orgID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org == nil {
		t.Fatal("expected org, got nil")
	}
	if org.ID != orgID {
		t.Errorf("expected ID %s, got %s", orgID, org.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &stubOrgRepo{
		getByIDFunc: func(ctx context.Context, id uuid.UUID) (*services.Organization, error) {
			return nil, services.ErrOrganizationNotFound
		},
	}
	service := services.NewOrganizationService(repo)

	_, err := service.GetByID(context.Background(), uuid.New())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, services.ErrOrganizationNotFound) {
		t.Errorf("expected ErrOrganizationNotFound, got: %v", err)
	}
}

func TestGetUserOrganizations_Success(t *testing.T) {
	repo := &stubOrgRepo{
		getUserOrganizationsFunc: func(ctx context.Context, userID int) ([]*services.Organization, error) {
			if userID != 1 {
				t.Errorf("expected userID 1, got %d", userID)
			}
			return []*services.Organization{
				{ID: uuid.New(), Name: "Org1"},
				{ID: uuid.New(), Name: "Org2"},
			}, nil
		},
	}
	service := services.NewOrganizationService(repo)

	orgs, err := service.GetUserOrganizations(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orgs == nil {
		t.Fatal("expected orgs, got nil")
	}
	if len(orgs) != 2 {
		t.Errorf("expected 2 orgs, got %d", len(orgs))
	}
}

func TestGetUserOrganizations_Empty(t *testing.T) {
	repo := &stubOrgRepo{
		getUserOrganizationsFunc: func(ctx context.Context, userID int) ([]*services.Organization, error) {
			return []*services.Organization{}, nil
		},
	}
	service := services.NewOrganizationService(repo)

	orgs, err := service.GetUserOrganizations(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orgs == nil {
		t.Fatal("expected orgs, got nil")
	}
	if len(orgs) != 0 {
		t.Errorf("expected 0 orgs, got %d", len(orgs))
	}
}

func TestGetActiveEnrollSecret_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &stubOrgRepo{
		getActiveEnrollSecretFunc: func(ctx context.Context, id uuid.UUID) (*services.OrganizationEnrollSecret, error) {
			if id != orgID {
				t.Errorf("expected orgID %s, got %s", orgID, id)
			}
			return &services.OrganizationEnrollSecret{Secret: "test-secret-abc123def456"}, nil
		},
	}
	service := services.NewOrganizationService(repo)

	secret, err := service.GetActiveEnrollSecret(context.Background(), orgID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "test-secret-abc123def456" {
		t.Errorf("expected secret 'test-secret-abc123def456', got %s", secret)
	}
}

func TestGetActiveEnrollSecret_None(t *testing.T) {
	orgID := uuid.New()
	repo := &stubOrgRepo{
		getActiveEnrollSecretFunc: func(ctx context.Context, id uuid.UUID) (*services.OrganizationEnrollSecret, error) {
			if id != orgID {
				t.Errorf("expected orgID %s, got %s", orgID, id)
			}
			return nil, nil
		},
	}
	service := services.NewOrganizationService(repo)

	secret, err := service.GetActiveEnrollSecret(context.Background(), orgID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "" {
		t.Errorf("expected empty secret, got %s", secret)
	}
}

func TestGetOrganizationByEnrollSecret_Success(t *testing.T) {
	orgID := uuid.New()
	repo := &stubOrgRepo{
		getOrgByEnrollSecretFunc: func(ctx context.Context, secret string) (*services.Organization, error) {
			if secret != "myorg-abc123" {
				t.Errorf("expected secret 'myorg-abc123', got %s", secret)
			}
			return &services.Organization{ID: orgID, Name: "MyOrg"}, nil
		},
	}
	service := services.NewOrganizationService(repo)

	org, err := service.GetOrganizationByEnrollSecret(context.Background(), "myorg-abc123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org == nil {
		t.Fatal("expected org, got nil")
	}
	if org.ID != orgID {
		t.Errorf("expected ID %s, got %s", orgID, org.ID)
	}
}

func TestGetOrganizationByEnrollSecret_NotFound(t *testing.T) {
	repo := &stubOrgRepo{
		getOrgByEnrollSecretFunc: func(ctx context.Context, secret string) (*services.Organization, error) {
			return nil, services.ErrOrganizationNotFound
		},
	}
	service := services.NewOrganizationService(repo)

	_, err := service.GetOrganizationByEnrollSecret(context.Background(), "bad-secret")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, services.ErrOrganizationNotFound) {
		t.Errorf("expected ErrOrganizationNotFound, got: %v", err)
	}
}
