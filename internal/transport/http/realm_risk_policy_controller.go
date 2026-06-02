package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// RealmRiskPolicyController exposes a realm's login-risk policy override.
type RealmRiskPolicyController struct {
	policies app.RiskPolicyUsecase
}

func NewRealmRiskPolicyController(policies app.RiskPolicyUsecase) kitrest.Controller {
	return &RealmRiskPolicyController{policies: policies}
}

func (c *RealmRiskPolicyController) Routes(r kitrest.Router) {
	r.Route("/api/realm-risk-policies", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.set, decodeSetRiskPolicy, identityRealmRiskPolicyDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Set a realm risk policy"), openapi.Tags("risk"), openapi.Errors(400)),
		))
		r.Get("/{realmId}", http.HandlerFunc(c.get))
	})
}

func (c *RealmRiskPolicyController) set(ctx context.Context, in api.RealmRiskPolicyDTO) (*api.RealmRiskPolicyDTO, error) {
	realmID, policy := api.RiskPolicyFromDTO(&in)
	saved, err := c.policies.SetPolicy(ctx, realmID, policy)
	if err != nil {
		return nil, err
	}
	return api.RealmRiskPolicyToDTO(saved), nil
}

func decodeSetRiskPolicy(req *http.Request) (api.RealmRiskPolicyDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.RealmRiskPolicyDTO](req)
	if err != nil {
		return api.RealmRiskPolicyDTO{}, err
	}
	return *body, nil
}

func (c *RealmRiskPolicyController) get(w http.ResponseWriter, r *http.Request) {
	realmID := r.PathValue("realmId")
	policy, err := c.policies.GetPolicy(r.Context(), realmID)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.RiskPolicyValueToDTO(realmID, policy))
}

func identityRealmRiskPolicyDTO(dto *api.RealmRiskPolicyDTO) *api.RealmRiskPolicyDTO { return dto }
