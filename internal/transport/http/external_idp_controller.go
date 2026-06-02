package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ExternalIDPController exposes /api/external-idps as JSON:API. The upstream
// client_secret is accepted on POST only and is never serialized on reads.
type ExternalIDPController struct {
	idps app.ExternalIDPConfigUsecase
}

func NewExternalIDPController(idps app.ExternalIDPConfigUsecase) kitrest.Controller {
	return &ExternalIDPController{idps: idps}
}

type createExternalIDPCommand struct {
	Config domain.ExternalIDPConfig
	Secret string
}

func (c *ExternalIDPController) Routes(r kitrest.Router) {
	r.Route("/api/external-idps", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.create, decodeCreateExternalIDP, api.ExternalIDPToReadDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Register an external IdP config"),
				openapi.Description("Stores per-realm upstream IdP config; clientSecret is envelope-encrypted and never returned."),
				openapi.Tags("external-idps"),
				openapi.Errors(400, 409),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.idps, api.ExternalIDPToReadDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List external IdP configs"), openapi.Tags("external-idps")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.idps, api.ExternalIDPToReadDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get an external IdP config"), openapi.Tags("external-idps"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.idps, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete an external IdP config"), openapi.Tags("external-idps"), openapi.Errors(404)),
			))
		})
	})
}

func (c *ExternalIDPController) create(ctx context.Context, cmd createExternalIDPCommand) (domain.ExternalIDPConfig, error) {
	return c.idps.CreateWithSecret(ctx, cmd.Config, cmd.Secret)
}

func decodeCreateExternalIDP(req *http.Request) (createExternalIDPCommand, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.ExternalIDPDTO](req)
	if err != nil {
		return createExternalIDPCommand{}, err
	}
	return createExternalIDPCommand{
		Config: api.ExternalIDPFromDTO(body),
		Secret: body.RSecret,
	}, nil
}
