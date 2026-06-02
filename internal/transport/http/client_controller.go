package http

import (
	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// ClientController exposes the OIDC clients resource at /api/clients as
// JSON:API. Admin RBAC enforcement lands with the Wave 10 admin API; until
// then deployments gate it at the network layer.
type ClientController struct {
	clients app.ClientUsecase
}

func NewClientController(clients app.ClientUsecase) kitrest.Controller {
	return &ClientController{clients: clients}
}

func (c *ClientController) Routes(r kitrest.Router) {
	r.Route("/api/clients", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCreateHandler(
			c.clients, api.ClientFromDTO, api.ClientToCreateDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Register an OIDC client"),
				openapi.Description("Creates a client; confidential clients return the secret once in clientSecret."),
				openapi.Tags("clients"),
				openapi.Errors(400, 409),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.clients, api.ClientToReadDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List OIDC clients"), openapi.Tags("clients")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.clients, api.ClientToReadDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get an OIDC client"), openapi.Tags("clients"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.clients, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete an OIDC client"), openapi.Tags("clients"), openapi.Errors(404)),
			))
		})
	})
}
