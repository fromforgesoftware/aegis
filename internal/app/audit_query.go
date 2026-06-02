package app

import (
	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// AuditEventReader queries the audit log (filtered, newest-first).
type AuditEventReader interface {
	repository.Lister[domain.AuditEvent]
}

// AuditQueryUsecase is the read-only admin surface over the audit log.
type AuditQueryUsecase interface {
	repository.Lister[domain.AuditEvent]
}

type auditQueryUsecase struct {
	usecase.Lister[domain.AuditEvent]
}

func NewAuditQueryUsecase(reader AuditEventReader) AuditQueryUsecase {
	return &auditQueryUsecase{Lister: usecase.NewLister(reader)}
}
