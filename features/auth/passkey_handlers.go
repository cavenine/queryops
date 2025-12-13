package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"queryops/features/auth/services"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/protocol"
)

// PasskeyHandlers contains the HTTP handlers for passkey (WebAuthn) authentication.
type PasskeyHandlers struct {
	webauthnService *services.WebAuthnService
	userService     *services.UserService
	sessionManager  *scs.SessionManager
}

// NewPasskeyHandlers creates a new PasskeyHandlers instance.
func NewPasskeyHandlers(
	webauthnService *services.WebAuthnService,
	userService *services.UserService,
	sessionManager *scs.SessionManager,
) *PasskeyHandlers {
	return &PasskeyHandlers{
		webauthnService: webauthnService,
		userService:     userService,
		sessionManager:  sessionManager,
	}
}

// jsonError writes an error response as JSON.
func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		slog.Error("failed to encode JSON error response", "error", err)
	}
}

// jsonSuccess writes a success response as JSON.
func jsonSuccess(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// RegisterBegin starts the passkey registration process.
// Requires the user to be authenticated.
// POST /passkey/register/begin
func (h *PasskeyHandlers) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the authenticated user from context
	user := GetUserFromContext(ctx)
	if user == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Begin the registration ceremony
	options, err := h.webauthnService.BeginRegistration(ctx, user)
	if err != nil {
		jsonError(w, "Failed to start registration: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the PublicKeyCredentialCreationOptions directly (not wrapped in {publicKey: ...})
	// SimpleWebAuthn expects the options object directly
	jsonSuccess(w, options.Response)
}

// RegisterFinish completes the passkey registration process.
// Requires the user to be authenticated.
// POST /passkey/register/finish
func (h *PasskeyHandlers) RegisterFinish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the authenticated user from context
	user := GetUserFromContext(ctx)
	if user == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Parse the credential creation response from the browser
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// First, extract the nickname if present (it's added as a custom field by the frontend)
	var requestData struct {
		Nickname string `json:"nickname"`
	}
	_ = json.Unmarshal(body, &requestData) // Ignore error - nickname is optional

	// Parse the raw attestation response
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	if err != nil {
		jsonError(w, "Failed to parse credential response: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Finish the registration ceremony with nickname
	credential, err := h.webauthnService.FinishRegistrationWithNickname(ctx, user, parsedResponse, requestData.Nickname)
	if err != nil {
		jsonError(w, "Failed to complete registration: "+err.Error(), http.StatusBadRequest)
		return
	}

	jsonSuccess(w, map[string]any{
		"success":      true,
		"credentialId": credential.ID,
	})
}

// LoginBegin starts the passkey login process (discoverable/usernameless).
// Public endpoint - does not require authentication.
// POST /passkey/login/begin
func (h *PasskeyHandlers) LoginBegin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Begin discoverable login (usernameless)
	options, err := h.webauthnService.BeginDiscoverableLogin(ctx)
	if err != nil {
		jsonError(w, "Failed to start login: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the PublicKeyCredentialRequestOptions directly (not wrapped in {publicKey: ...})
	// SimpleWebAuthn expects the options object directly
	jsonSuccess(w, options.Response)
}

// LoginFinish completes the passkey login process.
// Public endpoint - creates a session on success.
// POST /passkey/login/finish
func (h *PasskeyHandlers) LoginFinish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse the credential assertion response from the browser
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse the raw assertion response
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		jsonError(w, "Failed to parse credential response: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Finish the login ceremony
	user, err := h.webauthnService.FinishDiscoverableLogin(ctx, parsedResponse)
	if err != nil {
		jsonError(w, "Failed to complete login: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Create session for the authenticated user
	if err := SetSessionUserID(ctx, h.sessionManager, user.ID); err != nil {
		jsonError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]any{
		"success":  true,
		"redirect": "/",
	})
}
