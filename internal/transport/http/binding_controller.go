package http

import (
	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// BindingController exposes /api/bindings as JSON:API — the surface that
// grants a subject (account or group) a role on a resource. Deleting a
// binding revokes the grant.
type BindingController struct {
	bindings app.BindingUsecase
}

func NewBindingController(bindings app.BindingUsecase) kitrest.Controller {
	return &BindingController{bindings: bindings}
}

func (c *BindingController) Routes(r kitrest.Router) {
	r.Route("/api/bindings", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCreateHandler(
			c.bindings, api.BindingFromDTO, api.BindingToDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Grant a binding"),
				openapi.Description("Ties a subject (account or group) to a role on a resource; the role's resource_type must match the resource's type."),
				openapi.Tags("authz"), openapi.Errors(400, 409),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.bindings, api.BindingToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List bindings"), openapi.Tags("authz")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.bindings, api.BindingToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a binding"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.bindings, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Revoke a binding"), openapi.Tags("authz"), openapi.Errors(404)),
			))
		})
	})
}
