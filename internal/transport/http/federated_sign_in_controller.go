package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// FederatedSignInController exposes the identity-broker as a JSON:API
// operation at POST /api/auth/federate. Clients hand us the upstream IdP
// name + raw token; we verify, JIT-provision when needed, and return the
// resolved account plus link/created flags.
type FederatedSignInController struct {
	broker app.IdentityBrokerUsecase
	rl     kitrest.Middleware
}

func NewFederatedSignInController(broker app.IdentityBrokerUsecase, rl *kitrest.RateLimitMiddleware) kitrest.Controller {
	return &FederatedSignInController{broker: broker, rl: rl}
}

func (c *FederatedSignInController) Routes(r kitrest.Router) {
	r.With(c.rl).Post("/api/auth/federate", kitrest.NewJsonApiCommandHandler(
		c.federate, decodeFederate, identityFederatedSignInDTO,
		kitrest.HandlerWithSuccessStatus(http.StatusOK),
		kitrest.HandlerWithOpenAPI(
			openapi.Summary("Verify an upstream IdP token and resolve the account"),
			openapi.Description("JIT-provisions a new account or auto-links a verified-email collision; returns linkRequired=true when an explicit linking flow is needed."),
			openapi.Tags("identity-broker"),
			openapi.Errors(400, 401),
		),
	))
}

// federate runs the broker and pre-maps the result to the JSON:API DTO so the
// kit's command handler — which requires its R to be a resource.Resource —
// gets a value it can marshal directly.
func (c *FederatedSignInController) federate(ctx context.Context, in app.ResolveAccountInput) (*api.FederatedSignInDTO, error) {
	res, err := c.broker.ResolveAccount(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.FederatedSignInToDTO(res), nil
}

// identityFederatedSignInDTO is the encoder identity — the endpoint already
// returns the DTO, so the kit's command-handler encoder slot just passes
// through.
func identityFederatedSignInDTO(dto *api.FederatedSignInDTO) *api.FederatedSignInDTO { return dto }

func decodeFederate(req *http.Request) (app.ResolveAccountInput, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.FederatedSignInRequestDTO](req)
	if err != nil {
		return app.ResolveAccountInput{}, err
	}
	return app.ResolveAccountInput{
		RealmID:  body.RRealmID,
		IDPName:  body.RIDPName,
		RawToken: body.RToken,
	}, nil
}
