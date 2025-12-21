package pubsub

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
)

type recordingHandler struct {
	mu        sync.Mutex
	baseAttrs []slog.Attr
	last      recorded
}

type recorded struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	attrs := make(map[string]any)
	for _, a := range h.baseAttrs {
		attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	h.last = recorded{level: r.Level, msg: r.Message, attrs: attrs}
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()

	merged := make([]slog.Attr, 0, len(h.baseAttrs)+len(attrs))
	merged = append(merged, h.baseAttrs...)
	merged = append(merged, attrs...)
	return &recordingHandler{baseAttrs: merged}
}

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

func TestSlogAdapter_WithMergesFields(t *testing.T) {
	h := &recordingHandler{}
	logger := slog.New(h)
	adapter := NewSlogAdapter(logger)

	with := adapter.With(watermill.LogFields{"a": 1}).(*SlogAdapter)
	with.Info("hello", watermill.LogFields{"b": "x", "a": 2})

	h.mu.Lock()
	got := h.last
	h.mu.Unlock()

	if got.msg != "hello" {
		t.Fatalf("msg = %q, want hello", got.msg)
	}
	if got.attrs["a"] != int64(2) && got.attrs["a"] != 2 {
		t.Fatalf("a = %#v, want 2", got.attrs["a"])
	}
	if got.attrs["b"] != "x" {
		t.Fatalf("b = %#v, want x", got.attrs["b"])
	}
}

func TestSlogAdapter_ErrorIncludesErr(t *testing.T) {
	h := &recordingHandler{}
	logger := slog.New(h)
	adapter := NewSlogAdapter(logger)

	errExample := errors.New("example")
	adapter.Error("boom", errExample, nil)

	h.mu.Lock()
	got := h.last
	h.mu.Unlock()

	if got.level != slog.LevelError {
		t.Fatalf("level = %v, want error", got.level)
	}
	if got.attrs["error"] == nil {
		t.Fatalf("expected error attr")
	}
	if got.attrs["error"].(error).Error() != "example" {
		t.Fatalf("error = %v, want example", got.attrs["error"])
	}
}

func TestSlogAdapter_TraceMapsToDebug(t *testing.T) {
	h := &recordingHandler{}
	logger := slog.New(h)
	adapter := NewSlogAdapter(logger)

	adapter.Trace("trace", nil)

	h.mu.Lock()
	got := h.last
	h.mu.Unlock()

	if got.level != slog.LevelDebug {
		t.Fatalf("level = %v, want debug", got.level)
	}
}
