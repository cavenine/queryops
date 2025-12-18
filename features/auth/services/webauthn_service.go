package services

import (
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/cavenine/queryops/config"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func init() {
	// Register WebAuthn session data type for gob encoding (used by SCS)
	gob.Register(&webauthn.SessionData{})
}

const (
	// Session keys for storing WebAuthn ceremony data
	webauthnSessionKey = "webauthn_session"
)

// WebAuthnService handles WebAuthn registration and authentication ceremonies.
type WebAuthnService struct {
	webAuthn       *webauthn.WebAuthn
	credentialRepo *CredentialRepository
	userRepo       *UserRepository
	sessionManager *scs.SessionManager
}

// NewWebAuthnService creates a new WebAuthnService.
func NewWebAuthnService(
	cfg *config.Config,
	credentialRepo *CredentialRepository,
	userRepo *UserRepository,
	sessionManager *scs.SessionManager,
) (*WebAuthnService, error) {
	wconfig := &webauthn.Config{
		RPDisplayName: cfg.WebAuthnRPDisplayName,
		RPID:          cfg.WebAuthnRPID,
		RPOrigins:     []string{cfg.WebAuthnRPOrigin},
	}

	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("creating webauthn: %w", err)
	}

	return &WebAuthnService{
		webAuthn:       webAuthn,
		credentialRepo: credentialRepo,
		userRepo:       userRepo,
		sessionManager: sessionManager,
	}, nil
}

// BeginRegistration starts the WebAuthn registration ceremony for a user.
// Returns the options to pass to the browser's navigator.credentials.create().
func (s *WebAuthnService) BeginRegistration(ctx context.Context, user *User) (*protocol.CredentialCreation, error) {
	// Load existing credentials to exclude them from registration
	credentials, err := s.credentialRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("loading credentials: %w", err)
	}
	user.Credentials = credentials

	// Generate registration options
	options, session, err := s.webAuthn.BeginRegistration(user)
	if err != nil {
		return nil, fmt.Errorf("beginning registration: %w", err)
	}

	// Store session data for FinishRegistration
	s.sessionManager.Put(ctx, webauthnSessionKey, session)

	return options, nil
}

// FinishRegistration completes the WebAuthn registration ceremony.
// Verifies the attestation response and stores the new credential.
func (s *WebAuthnService) FinishRegistration(ctx context.Context, user *User, response *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	return s.FinishRegistrationWithNickname(ctx, user, response, "")
}

// FinishRegistrationWithNickname completes the WebAuthn registration ceremony with an optional nickname.
// Verifies the attestation response and stores the new credential.
func (s *WebAuthnService) FinishRegistrationWithNickname(ctx context.Context, user *User, response *protocol.ParsedCredentialCreationData, nickname string) (*webauthn.Credential, error) {
	// Retrieve session data
	sessionData, ok := s.sessionManager.Get(ctx, webauthnSessionKey).(*webauthn.SessionData)
	if !ok || sessionData == nil {
		return nil, fmt.Errorf("no registration session found")
	}

	// Clear session data regardless of outcome
	s.sessionManager.Remove(ctx, webauthnSessionKey)

	// Load existing credentials
	credentials, err := s.credentialRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("loading credentials: %w", err)
	}
	user.Credentials = credentials

	// Verify the attestation response
	credential, err := s.webAuthn.CreateCredential(user, *sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("creating credential: %w", err)
	}

	// Store the new credential with nickname
	if err := s.credentialRepo.CreateWithNickname(ctx, user.ID, *credential, nickname); err != nil {
		return nil, fmt.Errorf("storing credential: %w", err)
	}

	return credential, nil
}

// BeginDiscoverableLogin starts a usernameless/discoverable WebAuthn authentication.
// The browser will show all available passkeys for this site.
func (s *WebAuthnService) BeginDiscoverableLogin(ctx context.Context) (*protocol.CredentialAssertion, error) {
	options, session, err := s.webAuthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, fmt.Errorf("beginning discoverable login: %w", err)
	}

	// Store session data for FinishLogin
	s.sessionManager.Put(ctx, webauthnSessionKey, session)

	return options, nil
}

// FinishDiscoverableLogin completes a discoverable/usernameless WebAuthn authentication.
// Returns the authenticated user.
func (s *WebAuthnService) FinishDiscoverableLogin(ctx context.Context, response *protocol.ParsedCredentialAssertionData) (*User, error) {
	// Retrieve session data
	sessionData, ok := s.sessionManager.Get(ctx, webauthnSessionKey).(*webauthn.SessionData)
	if !ok || sessionData == nil {
		return nil, fmt.Errorf("no login session found")
	}

	// Clear session data regardless of outcome
	s.sessionManager.Remove(ctx, webauthnSessionKey)

	// Handler to find user by credential ID during discoverable login
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		cred, user, err := s.credentialRepo.GetByCredentialID(ctx, rawID)
		if err != nil {
			return nil, fmt.Errorf("looking up credential: %w", err)
		}
		if cred == nil || user == nil {
			return nil, fmt.Errorf("credential not found")
		}

		// Load all credentials for this user
		credentials, err := s.credentialRepo.GetByUserID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("loading user credentials: %w", err)
		}
		user.Credentials = credentials

		return user, nil
	}

	// Verify the assertion
	credential, err := s.webAuthn.ValidateDiscoverableLogin(handler, *sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("validating login: %w", err)
	}

	// Update sign count to detect cloned authenticators
	if err := s.credentialRepo.UpdateSignCount(ctx, credential.ID, credential.Authenticator.SignCount); err != nil {
		// Log but don't fail - sign count update is not critical
		slog.Error("failed to update passkey sign count", "error", err)
	}

	// Look up the user again to return
	_, user, err := s.credentialRepo.GetByCredentialID(ctx, credential.ID)
	if err != nil {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	return user, nil
}
