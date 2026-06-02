package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// RiskController exposes the login-risk assessment surface. A flow/gateway
// posts a login context and acts on the decision (allow / step-up / deny).
type RiskController struct {
	risk app.RiskUsecase
}

func NewRiskController(risk app.RiskUsecase) kitrest.Controller {
	return &RiskController{risk: risk}
}

func (c *RiskController) Routes(r kitrest.Router) {
	r.Route("/api/risk", func(r kitrest.Router) {
		r.Post("/assess", kitrest.NewJsonApiCommandHandler(
			c.assess, decodeRiskAssess, identityRiskAssessmentDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Assess login risk"),
				openapi.Description("Scores a login context (new IP/device, recent failures) and returns allow/step-up/deny."),
				openapi.Tags("risk"), openapi.Errors(400),
			),
		))
	})
}

func (c *RiskController) assess(ctx context.Context, in api.RiskAssessRequestDTO) (*api.RiskAssessmentDTO, error) {
	assessment, err := c.risk.Assess(ctx, app.RiskInput{
		RealmID: in.RRealmID, AccountID: in.RAccountID, IP: in.RIP, DeviceID: in.RDeviceID, Succeeded: in.RSucceeded,
	})
	if err != nil {
		return nil, err
	}
	return api.RiskAssessmentToDTO(assessment), nil
}

func decodeRiskAssess(req *http.Request) (api.RiskAssessRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.RiskAssessRequestDTO](req)
	if err != nil {
		return api.RiskAssessRequestDTO{}, err
	}
	return *body, nil
}

func identityRiskAssessmentDTO(dto *api.RiskAssessmentDTO) *api.RiskAssessmentDTO { return dto }
