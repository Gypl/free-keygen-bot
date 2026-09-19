package logging

import (
	"context"
	"log/slog"
	"time"
)

// AuditEvent represents a security or operational event to be recorded in audit logs.
type AuditEvent struct {
	ActorID  int64         `json:"actor_id"`
	Username string        `json:"username,omitempty"`
	Action   string        `json:"action"`
	Status   string        `json:"status"`
	Reason   string        `json:"reason,omitempty"`
	Duration time.Duration `json:"duration_ms,omitempty"`
}

// AuditLogger wraps an slog.Logger to emit structured audit entries.
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger returns a new AuditLogger with log_type=audit pre-populated.
func NewAuditLogger(base *slog.Logger) *AuditLogger {
	return &AuditLogger{
		logger: base.With(slog.String("log_type", "audit")),
	}
}

// Log writes an audit event as a structured log entry.
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	attrs := []slog.Attr{
		slog.Int64("actor_id", event.ActorID),
		slog.String("username", event.Username),
		slog.String("action", event.Action),
		slog.String("status", event.Status),
	}
	if event.Reason != "" {
		attrs = append(attrs, slog.String("reason", event.Reason))
	}
	if event.Duration > 0 {
		attrs = append(attrs, slog.Duration("duration_ms", event.Duration))
	}

	anyList := make([]any, len(attrs))
	for i, attr := range attrs {
		anyList[i] = attr
	}

	a.logger.LogAttrs(ctx, slog.LevelInfo, "audit_event", slog.Group("audit", anyList...))
}
