package http

import (
	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// AuditEventController exposes the read-only audit log at /api/audit-events,
// filterable by actor/action/resource (newest first). Backed by the built-in
// Postgres sink; Foundry renders the timeline against it.
type AuditEventController struct {
	audit app.AuditQueryUsecase
}

func NewAuditEventController(audit app.AuditQueryUsecase) kitrest.Controller {
	return &AuditEventController{audit: audit}
}

func (c *AuditEventController) Routes(r kitrest.Router) {
	r.Route("/api/audit-events", func(r kitrest.Router) {
		r.Get("", kitrest.NewJsonApiListHandler(
			c.audit, api.AuditEventToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Query the audit log"), openapi.Tags("admin")),
		))
	})
}
