package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cavenine/queryops/features/auth"
	"github.com/cavenine/queryops/features/auth/services"
	"github.com/cavenine/queryops/internal/antibot"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/go-chi/chi/v5"
)

type stubUserService struct {
	registerCalls int
}

func (s *stubUserService) Authenticate(_ context.Context, _, _ string) (*services.User, error) {
	return nil, services.ErrUserNotFound
}

func (s *stubUserService) Register(_ context.Context, email, _ string) (*services.User, error) {
	s.registerCalls++
	return &services.User{ID: 123, Email: email}, nil
}

func TestRegisterSubmit_AntibotBlocked_NoSideEffects(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	sm := scs.New()
	sm.Store = memstore.New()

	us := &stubUserService{}
	h := auth.NewHandlers(us, sm)
	h.SetAntibot(antibot.New(sm, antibot.Config{
		MinDelay:  2 * time.Second,
		MaxTokens: 5,
		Now: func() time.Time {
			return now
		},
	}))

	r := chi.NewRouter()
	r.Use(sm.LoadAndSave)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.RegisterSubmit)

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/register", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /register status = %d", getRec.Code)
	}

	cookie := getRec.Result().Cookies()[0]

	// Extract token emitted into HTML.
	m := regexp.MustCompile(`data-antibot-token="([^"]+)"`).FindStringSubmatch(getRec.Body.String())
	if len(m) != 2 {
		t.Fatalf("expected antibot token in register page")
	}
	token := m[1]

	now = now.Add(3 * time.Second)

	form := url.Values{}
	form.Set("email", "a@example.com")
	form.Set("password", "password")
	form.Set("website", "bot") // honeypot hit
	form.Set("js_token", token)

	postReq := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(cookie)

	postRec := httptest.NewRecorder()
	r.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /register status = %d", postRec.Code)
	}
	if us.registerCalls != 0 {
		t.Fatalf("expected no Register calls, got %d", us.registerCalls)
	}
}

func TestRegisterSubmit_AntibotAllowed_TriggersRegister(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	sm := scs.New()
	sm.Store = memstore.New()

	us := &stubUserService{}
	h := auth.NewHandlers(us, sm)
	h.SetAntibot(antibot.New(sm, antibot.Config{
		MinDelay:  2 * time.Second,
		MaxTokens: 5,
		Now: func() time.Time {
			return now
		},
	}))

	r := chi.NewRouter()
	r.Use(sm.LoadAndSave)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.RegisterSubmit)

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/register", nil))
	cookie := getRec.Result().Cookies()[0]

	m := regexp.MustCompile(`data-antibot-token="([^"]+)"`).FindStringSubmatch(getRec.Body.String())
	if len(m) != 2 {
		t.Fatalf("expected antibot token in register page")
	}
	token := m[1]

	now = now.Add(3 * time.Second)

	form := url.Values{}
	form.Set("email", "a@example.com")
	form.Set("password", "password")
	form.Set("website", "")
	form.Set("js_token", token)

	postReq := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(cookie)

	postRec := httptest.NewRecorder()
	r.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusSeeOther {
		t.Fatalf("POST /register status = %d", postRec.Code)
	}
	if us.registerCalls != 1 {
		t.Fatalf("expected 1 Register call, got %d", us.registerCalls)
	}

	loc := postRec.Header().Get("Location")
	if loc != "/" {
		t.Fatalf("expected redirect to /, got %q", loc)
	}
}
