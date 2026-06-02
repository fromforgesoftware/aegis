package http

import (
	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// RealmController exposes /api/realms CRUD — the top-level admin resource.
type RealmController struct {
	realms app.RealmUsecase
}

func NewRealmController(realms app.RealmUsecase) kitrest.Controller {
	return &RealmController{realms: realms}
}

func (c *RealmController) Routes(r kitrest.Router) {
	r.Route("/api/realms", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCreateHandler(
			c.realms, api.RealmFromDTO, api.RealmToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Create a realm"), openapi.Tags("admin"), openapi.Errors(400, 409)),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.realms, api.RealmToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List realms"), openapi.Tags("admin")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.realms, api.RealmToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a realm"), openapi.Tags("admin"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.realms, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete a realm"), openapi.Tags("admin"), openapi.Errors(404)),
			))
		})
	})
}
