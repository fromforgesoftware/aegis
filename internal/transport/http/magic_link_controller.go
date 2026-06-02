package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// MagicLinkController exposes passwordless email login: request a one-time
// link, then redeem it to authenticate.
type MagicLinkController struct {
	magic app.MagicLinkUsecase
}

func NewMagicLinkController(magic app.MagicLinkUsecase) kitrest.Controller {
	return &MagicLinkController{magic: magic}
}

func (c *MagicLinkController) Routes(r kitrest.Router) {
	r.Route("/api/magic-links", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.request, decodeMagicLinkRequest, identityMagicLinkAckDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusAccepted),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Request a magic login link"), openapi.Tags("magic-link"), openapi.Errors(400)),
		))
		r.Post("/redeem", kitrest.NewJsonApiCommandHandler(
			c.redeem, decodeMagicLinkRedeem, identityMagicLinkSessionDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Redeem a magic login link"), openapi.Tags("magic-link"), openapi.Errors(400, 401)),
		))
	})
}

func (c *MagicLinkController) request(ctx context.Context, in api.MagicLinkRequestDTO) (*api.MagicLinkAckDTO, error) {
	if err := c.magic.RequestMagicLink(ctx, in.RRealmID, in.REmail); err != nil {
		return nil, err
	}
	return api.MagicLinkAckToDTO(), nil
}

func (c *MagicLinkController) redeem(ctx context.Context, in api.MagicLinkRedeemDTO) (*api.MagicLinkSessionDTO, error) {
	acc, err := c.magic.RedeemMagicLink(ctx, in.RToken)
	if err != nil {
		return nil, err
	}
	return api.MagicLinkSessionToDTO(acc.ID(), acc.RealmID(), acc.Email()), nil
}

func decodeMagicLinkRequest(req *http.Request) (api.MagicLinkRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.MagicLinkRequestDTO](req)
	if err != nil {
		return api.MagicLinkRequestDTO{}, err
	}
	return *body, nil
}

func decodeMagicLinkRedeem(req *http.Request) (api.MagicLinkRedeemDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.MagicLinkRedeemDTO](req)
	if err != nil {
		return api.MagicLinkRedeemDTO{}, err
	}
	return *body, nil
}

func identityMagicLinkAckDTO(dto *api.MagicLinkAckDTO) *api.MagicLinkAckDTO             { return dto }
func identityMagicLinkSessionDTO(dto *api.MagicLinkSessionDTO) *api.MagicLinkSessionDTO { return dto }
