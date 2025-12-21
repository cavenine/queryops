package pubsub

import (
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
)

// SlogAdapter adapts slog.Logger to Watermill's LoggerAdapter interface.
type SlogAdapter struct {
	logger *slog.Logger
	fields watermill.LogFields
}

// NewSlogAdapter creates a new adapter.
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogAdapter{logger: logger}
}

func (s *SlogAdapter) Error(msg string, err error, fields watermill.LogFields) {
	s.logger.Error(msg, s.toArgs(fields, err)...)
}

func (s *SlogAdapter) Info(msg string, fields watermill.LogFields) {
	s.logger.Info(msg, s.toArgs(fields, nil)...)
}

func (s *SlogAdapter) Debug(msg string, fields watermill.LogFields) {
	s.logger.Debug(msg, s.toArgs(fields, nil)...)
}

func (s *SlogAdapter) Trace(msg string, fields watermill.LogFields) {
	// slog has no trace.
	s.logger.Debug(msg, s.toArgs(fields, nil)...)
}

func (s *SlogAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
	merged := make(watermill.LogFields, len(s.fields)+len(fields))
	for k, v := range s.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &SlogAdapter{logger: s.logger, fields: merged}
}

func (s *SlogAdapter) toArgs(fields watermill.LogFields, err error) []any {
	args := make([]any, 0, len(s.fields)+len(fields)+1)
	for k, v := range s.fields {
		args = append(args, k, v)
	}
	for k, v := range fields {
		args = append(args, k, v)
	}
	if err != nil {
		args = append(args, "error", err)
	}
	return args
}
