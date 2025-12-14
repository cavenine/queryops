package antibot

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

func TestProtector_Validate(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	sm := scs.New()
	sm.Store = memstore.New()

	p := New(sm, Config{
		MinDelay:  2 * time.Second,
		MaxTokens: 5,
		Now: func() time.Time {
			return now
		},
	})

	var token string
	issue := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, err := p.Issue(r.Context(), "register")
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		token = tok
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	issue.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/register", nil))

	cookie := rec.Result().Cookies()[0]

	t.Run("allow", func(t *testing.T) {
		now = now.Add(3 * time.Second)

		var res Result
		post := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res = p.Validate(r, "register", token, "")
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/register", nil)
		r.AddCookie(cookie)
		post.ServeHTTP(w, r)

		if !res.Allowed {
			t.Fatalf("expected allowed, got %q", res.Reason)
		}

		// Token is single-use.
		var res2 Result
		post2 := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res2 = p.Validate(r, "register", token, "")
			w.WriteHeader(http.StatusOK)
		}))

		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest(http.MethodPost, "/register", nil)
		r2.AddCookie(cookie)
		post2.ServeHTTP(w2, r2)

		if res2.Allowed || res2.Reason != ReasonTokenInvalid {
			t.Fatalf("expected token invalid after use, got allowed=%v reason=%q", res2.Allowed, res2.Reason)
		}
	})

	blockedCases := []struct {
		name          string
		useValidToken bool
		postedToken   string
		honeypot      string
		skipDelay     bool
		reason        Reason
	}{
		{name: "honeypot", useValidToken: true, honeypot: "spam", reason: ReasonHoneypot},
		{name: "missing_token", postedToken: "", honeypot: "", reason: ReasonTokenMissing},
		{name: "token_mismatch", postedToken: "bogus", honeypot: "", reason: ReasonTokenInvalid},
		{name: "too_fast", useValidToken: true, honeypot: "", skipDelay: true, reason: ReasonTooFast},
	}

	for _, tc := range blockedCases {
		t.Run(tc.name, func(t *testing.T) {
			now = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

			// Re-issue a token each time so tests are independent.
			var tok string
			rec := httptest.NewRecorder()
			issue.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/register", nil))
			cookie := rec.Result().Cookies()[0]
			tok = token

			if !tc.skipDelay {
				now = now.Add(3 * time.Second)
			}

			var res Result
			post := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				postedTok := tc.postedToken
				if tc.useValidToken {
					postedTok = tok
				}
				res = p.Validate(r, "register", postedTok, tc.honeypot)
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/register", nil)
			r.AddCookie(cookie)
			post.ServeHTTP(w, r)

			if res.Allowed {
				t.Fatalf("expected blocked")
			}
			if res.Reason != tc.reason {
				t.Fatalf("expected reason %q, got %q", tc.reason, res.Reason)
			}
		})
	}
}

func TestProtector_Issue_TrimsTokens(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	sm := scs.New()
	sm.Store = memstore.New()

	p := New(sm, Config{
		MinDelay:  time.Second,
		MaxTokens: 2,
		Now: func() time.Time {
			return now
		},
	})

	var tokensAfter map[string]int64
	h := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 3; i++ {
			_, err := p.Issue(r.Context(), "register")
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}
			now = now.Add(10 * time.Millisecond)
		}

		v := sm.Get(r.Context(), sessionKey("register"))
		m, ok := v.(map[string]int64)
		if !ok {
			t.Fatalf("expected map in session")
		}
		tokensAfter = m
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/register", nil))

	if len(tokensAfter) != 2 {
		t.Fatalf("expected 2 tokens after trim, got %d", len(tokensAfter))
	}
}
