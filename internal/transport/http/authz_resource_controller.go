package http

import (
	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// AuthzResourceController exposes /api/resources as JSON:API. Consumers
// register a row here when they create a domain object that participates in
// authz so ACL bindings have a target id.
type AuthzResourceController struct {
	resources app.AuthzResourceUsecase
}

func NewAuthzResourceController(resources app.AuthzResourceUsecase) kitrest.Controller {
	return &AuthzResourceController{resources: resources}
}

func (c *AuthzResourceController) Routes(r kitrest.Router) {
	r.Route("/api/resources", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCreateHandler(
			c.resources, api.AuthzResourceFromDTO, api.AuthzResourceToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Register a resource"), openapi.Tags("authz"), openapi.Errors(400, 409)),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.resources, api.AuthzResourceToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List resources"), openapi.Tags("authz")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.resources, api.AuthzResourceToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a resource"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.resources, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete a resource"), openapi.Tags("authz"), openapi.Errors(404)),
			))
		})
	})
}
