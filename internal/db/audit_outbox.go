package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/audit"
	"github.com/fromforgesoftware/go-kit/outbox"
)

// auditOutboxSink writes audit events to the transactional outbox; a drainer
// (shipped by talos) claims them and forwards to the telemetry store, so
// emission stays decoupled from any sink backend.
type auditOutboxSink struct {
	repo outbox.Repository
}

func NewAuditOutboxSink(repo outbox.Repository) *auditOutboxSink {
	return &auditOutboxSink{repo: repo}
}

func (s *auditOutboxSink) Emit(ctx context.Context, e audit.Event) error {
	draft, err := outbox.NewDraft("audit", e)
	if err != nil {
		return err
	}
	return s.repo.Enqueue(ctx, draft)
}
