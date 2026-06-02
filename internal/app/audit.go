package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/audit"
	"github.com/fromforgesoftware/go-kit/auth"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

// AuditSink is the audit emission port. Built-in implementations: the Postgres
// sink (default) and the stdout sink. Forwarding to telemetry (talos) is done
// out-of-process by a drainer over an outbox, so aegis imports no sink backend.
type AuditSink = audit.Sink

// Auditor builds audit events (filling the actor from the request token) and
// emits them best-effort: a sink failure is logged, never surfaced to the
// caller, so auditing never breaks the audited operation.
type Auditor interface {
	Record(ctx context.Context, action, resourceType, resourceID string, changes map[string]any)
}

type auditor struct {
	sink audit.Sink
	log  logger.Logger
}

func NewAuditor(sink audit.Sink) Auditor {
	return &auditor{sink: sink, log: logger.New()}
}

func (a *auditor) Record(ctx context.Context, action, resourceType, resourceID string, changes map[string]any) {
	e := audit.Event{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
	}
	if id := actorFromContext(ctx); id != "" {
		e.ActorID = id
		e.ActorType = "ACCOUNT"
	}
	if err := a.sink.Emit(ctx, e); err != nil {
		a.log.ErrorContext(ctx, "audit emit failed", "error", err, "action", action)
	}
}

func actorFromContext(ctx context.Context) string {
	tok := auth.TokenFromCtx(ctx)
	if tok == nil {
		return ""
	}
	return tok.Claims().Subject()
}

// NoopAuditor discards events; the default when no sink is configured.
type NoopAuditor struct{}

func (NoopAuditor) Record(context.Context, string, string, string, map[string]any) {}

// StdoutAuditSink logs events as structured log lines — the simplest sink and
// a useful default in development.
type StdoutAuditSink struct {
	log logger.Logger
}

func NewStdoutAuditSink() *StdoutAuditSink {
	return &StdoutAuditSink{log: logger.New()}
}

func (s *StdoutAuditSink) Emit(ctx context.Context, e audit.Event) error {
	s.log.InfoContext(ctx, "audit",
		"action", e.Action,
		"resourceType", e.ResourceType,
		"resourceId", e.ResourceID,
		"actorId", e.ActorID,
		"changes", e.Changes,
	)
	return nil
}
