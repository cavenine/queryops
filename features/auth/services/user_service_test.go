package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cavenine/queryops/features/auth/services"
	"golang.org/x/crypto/bcrypt"
)

type stubUserRepo struct {
	emailExistsFunc func(ctx context.Context, email string) (bool, error)
	createFunc      func(ctx context.Context, email, passwordHash string) (*services.User, error)
	getByEmailFunc  func(ctx context.Context, email string) (*services.User, error)
	getByIDFunc     func(ctx context.Context, id int) (*services.User, error)
}

func (s *stubUserRepo) EmailExists(ctx context.Context, email string) (bool, error) {
	if s.emailExistsFunc != nil {
		return s.emailExistsFunc(ctx, email)
	}
	return false, nil
}

func (s *stubUserRepo) Create(ctx context.Context, email, passwordHash string) (*services.User, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, email, passwordHash)
	}
	return nil, nil
}

func (s *stubUserRepo) GetByEmail(ctx context.Context, email string) (*services.User, error) {
	if s.getByEmailFunc != nil {
		return s.getByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (s *stubUserRepo) GetByID(ctx context.Context, id int) (*services.User, error) {
	if s.getByIDFunc != nil {
		return s.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func TestRegister_Success(t *testing.T) {
	repo := &stubUserRepo{
		emailExistsFunc: func(ctx context.Context, email string) (bool, error) {
			return false, nil
		},
		createFunc: func(ctx context.Context, email, passwordHash string) (*services.User, error) {
			return &services.User{ID: 1, Email: email, PasswordHash: passwordHash}, nil
		},
	}
	service := services.NewUserService(repo)

	user, err := service.Register(context.Background(), "test@example.com", "password123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.PasswordHash == "" {
		t.Error("expected password hash to be generated, got empty string")
	}
}

func TestRegister_EmptyEmail(t *testing.T) {
	repo := &stubUserRepo{}
	service := services.NewUserService(repo)

	_, err := service.Register(context.Background(), "", "password123")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "email is required" {
		t.Errorf("expected 'email is required' error, got: %v", err)
	}
}

func TestRegister_EmptyPassword(t *testing.T) {
	repo := &stubUserRepo{}
	service := services.NewUserService(repo)

	_, err := service.Register(context.Background(), "test@example.com", "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "password is required" {
		t.Errorf("expected 'password is required' error, got: %v", err)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := &stubUserRepo{
		emailExistsFunc: func(ctx context.Context, email string) (bool, error) {
			return true, nil
		},
	}
	service := services.NewUserService(repo)

	_, err := service.Register(context.Background(), "existing@example.com", "password123")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "email already registered" {
		t.Errorf("expected 'email already registered' error, got: %v", err)
	}
}

func TestAuthenticate_Success(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate hash: %v", err)
	}

	repo := &stubUserRepo{
		getByEmailFunc: func(ctx context.Context, email string) (*services.User, error) {
			return &services.User{ID: 1, Email: email, PasswordHash: string(hash)}, nil
		},
	}
	service := services.NewUserService(repo)

	user, err := service.Authenticate(context.Background(), "test@example.com", "password")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
}

func TestAuthenticate_EmptyCredentials(t *testing.T) {
	repo := &stubUserRepo{}
	service := services.NewUserService(repo)

	_, err := service.Authenticate(context.Background(), "", "password")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "email and password required" {
		t.Errorf("expected 'email and password required' error, got: %v", err)
	}
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	repo := &stubUserRepo{
		getByEmailFunc: func(ctx context.Context, email string) (*services.User, error) {
			return nil, services.ErrUserNotFound
		},
	}
	service := services.NewUserService(repo)

	_, err := service.Authenticate(context.Background(), "notfound@example.com", "password")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "invalid email or password" {
		t.Errorf("expected 'invalid email or password' error, got: %v", err)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate hash: %v", err)
	}

	repo := &stubUserRepo{
		getByEmailFunc: func(ctx context.Context, email string) (*services.User, error) {
			return &services.User{ID: 1, Email: email, PasswordHash: string(hash)}, nil
		},
	}
	service := services.NewUserService(repo)

	_, err = service.Authenticate(context.Background(), "test@example.com", "wrongpassword")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "invalid email or password" {
		t.Errorf("expected 'invalid email or password' error, got: %v", err)
	}
}

func TestGetByID_Success(t *testing.T) {
	repo := &stubUserRepo{
		getByIDFunc: func(ctx context.Context, id int) (*services.User, error) {
			return &services.User{ID: id, Email: "test@example.com"}, nil
		},
	}
	service := services.NewUserService(repo)

	user, err := service.GetByID(context.Background(), 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != 42 {
		t.Errorf("expected ID 42, got %d", user.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &stubUserRepo{
		getByIDFunc: func(ctx context.Context, id int) (*services.User, error) {
			return nil, services.ErrUserNotFound
		},
	}
	service := services.NewUserService(repo)

	_, err := service.GetByID(context.Background(), 999)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, services.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}
