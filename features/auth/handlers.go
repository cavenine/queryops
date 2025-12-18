package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/cavenine/queryops/features/auth/pages"
	"github.com/cavenine/queryops/features/auth/services"
	"github.com/cavenine/queryops/internal/antibot"

	"github.com/alexedwards/scs/v2"
)

const registerFormID = "register"

type userService interface {
	Authenticate(ctx context.Context, email, password string) (*services.User, error)
	Register(ctx context.Context, email, password string) (*services.User, error)
}

// Handlers contains the HTTP handlers for authentication.
type Handlers struct {
	userService    userService
	sessionManager *scs.SessionManager
	antibot        *antibot.Protector
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(userService userService, sessionManager *scs.SessionManager) *Handlers {
	return &Handlers{
		userService:    userService,
		sessionManager: sessionManager,
		antibot:        antibot.New(sessionManager, antibot.DefaultConfig()),
	}
}

// LoginPage renders the login form.
func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	if err := pages.LoginPage("", "").Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// LoginSubmit handles the login form submission.
func (h *Handlers) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderLoginError(w, r, "", "Invalid form data")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderLoginError(w, r, email, "Email and password are required")
		return
	}

	user, err := h.userService.Authenticate(r.Context(), email, password)
	if err != nil {
		h.renderLoginError(w, r, email, "Invalid email or password")
		return
	}

	if err := SetSessionUserID(r.Context(), h.sessionManager, user.ID); err != nil {
		h.renderLoginError(w, r, email, "Failed to create session")
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) renderLoginError(w http.ResponseWriter, r *http.Request, email, errorMsg string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := pages.LoginPage(email, errorMsg).Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// RegisterPage renders the registration form.
func (h *Handlers) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.renderRegisterForm(w, r, "", "")
}

// RegisterSubmit handles the registration form submission.
func (h *Handlers) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderRegisterError(w, r, "", "Invalid form data")
		return
	}

	ab := h.antibot.Validate(r, registerFormID, r.FormValue("js_token"), r.FormValue("website"))
	if !ab.Allowed {
		slog.Warn(
			"antibot blocked register",
			"reason", ab.Reason,
			"ip", antibot.ClientIP(r),
			"ua", r.UserAgent(),
		)
		h.renderRegisterError(w, r, r.FormValue("email"), "Unable to submit form. Please refresh and try again.")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderRegisterError(w, r, email, "Email and password are required")
		return
	}

	user, err := h.userService.Register(r.Context(), email, password)
	if err != nil {
		h.renderRegisterError(w, r, email, err.Error())
		return
	}

	// Auto-login after registration
	if err := SetSessionUserID(r.Context(), h.sessionManager, user.ID); err != nil {
		h.renderRegisterError(w, r, email, "Account created but failed to log in")
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) renderRegisterError(w http.ResponseWriter, r *http.Request, email, errorMsg string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	h.renderRegisterForm(w, r, email, errorMsg)
}

func (h *Handlers) renderRegisterForm(w http.ResponseWriter, r *http.Request, email, errorMsg string) {
	token, err := h.antibot.Issue(r.Context(), registerFormID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := pages.RegisterPage(email, errorMsg, token).Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// Logout clears the session and redirects to login.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	_ = ClearSession(r.Context(), h.sessionManager)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
