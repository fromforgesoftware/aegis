// Package http exposes Aegis's REST surface. The flow controller drives
// the API-first interactive auth flows (start → required-fields → submit)
// as a JSON:API resource, registered against the kit's REST gateway.
package http

import (
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// FlowController exposes the authFlows resource at /api/auth/flows.
type FlowController struct {
	flows app.FlowUsecase
	rl    kitrest.Middleware // per-IP rate limit on the credential-submit path
}

func NewFlowController(flows app.FlowUsecase, rl *kitrest.RateLimitMiddleware) kitrest.Controller {
	return &FlowController{flows: flows, rl: rl}
}

func (c *FlowController) Routes(r kitrest.Router) {
	r.Route("/api/auth/flows", func(r kitrest.Router) {
		// Start a flow (create the resource).
		r.Post("", kitrest.NewJsonApiCreateHandler(
			c.flows, api.FlowFromDTO, api.FlowToDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Start an auth flow"),
				openapi.Description("Opens an interactive flow (login/registration/recovery/verification); the response lists the fields to submit."),
				openapi.Tags("auth"),
				openapi.NoSecurity(),
			),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.flows, api.FlowToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(
					openapi.Summary("Get an auth flow"),
					openapi.Tags("auth"),
					openapi.NoSecurity(),
				),
			))
			// Submit advances the flow; it returns the (possibly completed)
			// flow rather than a new resource, so success is 200 not 201.
			// Per-IP rate limited — this is the credential-check vector.
			r.With(c.rl).Patch("", kitrest.NewJsonApiCommandHandler(
				c.flows.Submit, decodeSubmit, api.FlowToDTO,
				kitrest.HandlerWithSuccessStatus(http.StatusOK),
				kitrest.HandlerWithOpenAPI(
					openapi.Summary("Submit an auth flow"),
					openapi.Tags("auth"),
					openapi.NoSecurity(),
				),
			))
		})
	})
}

// decodeSubmit builds the submit command from the JSON:API body
// (per-field attributes) + the {id} path param.
func decodeSubmit(r *http.Request) (app.SubmitFlowInput, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.FlowSubmitDTO](r)
	if err != nil {
		return app.SubmitFlowInput{}, err
	}
	return app.SubmitFlowInput{FlowID: r.PathValue("id"), Payload: body.Payload()}, nil
}
