package account

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/cavenine/queryops/features/account/pages"
	"github.com/cavenine/queryops/features/auth"
	"github.com/cavenine/queryops/features/auth/services"

	"github.com/go-chi/chi/v5"
)

// Handlers contains the HTTP handlers for account management.
type Handlers struct {
	credentialRepo *services.CredentialRepository
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(credentialRepo *services.CredentialRepository) *Handlers {
	return &Handlers{
		credentialRepo: credentialRepo,
	}
}

// AccountPage renders the account settings page.
func (h *Handlers) AccountPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := auth.GetUserFromContext(ctx)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	passkeys, err := h.credentialRepo.GetPasskeysByUserID(ctx, user.ID)
	if err != nil {
		http.Error(w, "Failed to load passkeys", http.StatusInternalServerError)
		return
	}

	if err := pages.AccountPage(user.Email, passkeys).Render(ctx, w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// DeletePasskey removes a passkey.
// DELETE /account/passkey/{id}
func (h *Handlers) DeletePasskey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := auth.GetUserFromContext(ctx)
	if user == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Get credential ID from URL (base64 raw URL encoded, no padding)
	idParam := chi.URLParam(r, "id")
	credentialID, err := base64.RawURLEncoding.DecodeString(idParam)
	if err != nil {
		jsonError(w, "Invalid passkey ID", http.StatusBadRequest)
		return
	}

	// Check how many passkeys the user has
	count, err := h.credentialRepo.CountByUserID(ctx, user.ID)
	if err != nil {
		jsonError(w, "Failed to check passkey count", http.StatusInternalServerError)
		return
	}

	// Prevent removing the last passkey if user has no password
	// Allow removal if: user has a password OR has other passkeys
	if count <= 1 && !user.HasPassword() {
		jsonError(w, "Cannot remove your only passkey. Set a password or add another passkey first.", http.StatusBadRequest)
		return
	}

	// Delete the passkey (only if it belongs to this user)
	deleted, err := h.credentialRepo.DeleteByUserAndID(ctx, user.ID, credentialID)
	if err != nil {
		jsonError(w, "Failed to remove passkey", http.StatusInternalServerError)
		return
	}

	if !deleted {
		jsonError(w, "Passkey not found", http.StatusNotFound)
		return
	}

	jsonSuccess(w, map[string]bool{"success": true})
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
