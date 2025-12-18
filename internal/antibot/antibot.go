package antibot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"maps"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
)

func init() {
	// Register map type for gob encoding (used by SCS).
	// SCS stores values as interface{}, which requires registration.
	gob.Register(map[string]int64{})
}

type Reason string

const (
	ReasonNone         Reason = ""
	ReasonHoneypot     Reason = "honeypot"
	ReasonTokenMissing Reason = "token_missing"
	ReasonTokenInvalid Reason = "token_mismatch"
	ReasonTooFast      Reason = "too_fast"
)

type Result struct {
	Allowed bool
	Reason  Reason
}

type Config struct {
	// MinDelay is the minimum time between render and submit.
	MinDelay time.Duration

	// MaxTokens bounds the number of outstanding tokens per form.
	MaxTokens int

	Now func() time.Time
}

func DefaultConfig() Config {
	return Config{
		MinDelay:  2 * time.Second,
		MaxTokens: 5,
		Now:       time.Now,
	}
}

type Protector struct {
	sessionManager *scs.SessionManager
	cfg            Config
}

func New(sessionManager *scs.SessionManager, cfg Config) *Protector {
	if cfg.MinDelay <= 0 {
		cfg.MinDelay = DefaultConfig().MinDelay
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = DefaultConfig().MaxTokens
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return &Protector{
		sessionManager: sessionManager,
		cfg:            cfg,
	}
}

func (p *Protector) Issue(ctx context.Context, formID string) (string, error) {
	if strings.TrimSpace(formID) == "" {
		return "", errors.New("formID is required")
	}

	tokens := p.getTokens(ctx, formID)
	const tokenSize = 32
	token, err := newToken(tokenSize)
	if err != nil {
		return "", err
	}

	tokens[token] = p.cfg.Now().UnixMilli()
	trimToN(tokens, p.cfg.MaxTokens)
	p.sessionManager.Put(ctx, sessionKey(formID), tokens)

	return token, nil
}

func (p *Protector) Validate(r *http.Request, formID string, postedToken string, honeypotValue string) Result {
	ctx := r.Context()
	if strings.TrimSpace(honeypotValue) != "" {
		return Result{Allowed: false, Reason: ReasonHoneypot}
	}

	postedToken = strings.TrimSpace(postedToken)
	if postedToken == "" {
		return Result{Allowed: false, Reason: ReasonTokenMissing}
	}

	tokens := p.getTokens(ctx, formID)
	renderedAtMs, ok := tokens[postedToken]
	if !ok {
		return Result{Allowed: false, Reason: ReasonTokenInvalid}
	}

	elapsed := p.cfg.Now().Sub(time.UnixMilli(renderedAtMs))
	if elapsed < p.cfg.MinDelay {
		return Result{Allowed: false, Reason: ReasonTooFast}
	}

	// Single-use token: remove on success.
	delete(tokens, postedToken)
	p.sessionManager.Put(ctx, sessionKey(formID), tokens)

	return Result{Allowed: true, Reason: ReasonNone}
}

func ClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first value.
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" {
		return xri
	}

	return r.RemoteAddr
}

func (p *Protector) getTokens(ctx context.Context, formID string) map[string]int64 {
	key := sessionKey(formID)
	val := p.sessionManager.Get(ctx, key)
	if val == nil {
		return map[string]int64{}
	}

	tokens, ok := val.(map[string]int64)
	if !ok || tokens == nil {
		return map[string]int64{}
	}

	// Defensive copy: avoid mutating a shared map.
	cpy := make(map[string]int64, len(tokens))
	maps.Copy(cpy, tokens)
	return cpy
}

func sessionKey(formID string) string {
	return "antibot." + formID
}

func newToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func trimToN(tokens map[string]int64, maxN int) {
	if maxN <= 0 {
		return
	}
	if len(tokens) <= maxN {
		return
	}

	type pair struct {
		token string
		ms    int64
	}
	pairs := make([]pair, 0, len(tokens))
	for t, ms := range tokens {
		pairs = append(pairs, pair{token: t, ms: ms})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].ms < pairs[j].ms })

	removeN := len(tokens) - maxN
	for i := range removeN {
		delete(tokens, pairs[i].token)
	}
}
