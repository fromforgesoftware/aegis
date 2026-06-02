package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// RealmACRPolicyController exposes a realm's MFA / assurance policy.
type RealmACRPolicyController struct {
	policies app.MFAPolicyUsecase
}

func NewRealmACRPolicyController(policies app.MFAPolicyUsecase) kitrest.Controller {
	return &RealmACRPolicyController{policies: policies}
}

func (c *RealmACRPolicyController) Routes(r kitrest.Router) {
	r.Route("/api/realm-acr-policies", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.set, decodeSetACRPolicy, api.RealmACRPolicyToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Set a realm MFA policy"), openapi.Tags("mfa"), openapi.Errors(400)),
		))
		r.Get("/{realmId}", http.HandlerFunc(c.get))
	})
}

func (c *RealmACRPolicyController) set(ctx context.Context, p domain.RealmACRPolicy) (domain.RealmACRPolicy, error) {
	return c.policies.SetPolicy(ctx, p)
}

func decodeSetACRPolicy(req *http.Request) (domain.RealmACRPolicy, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.RealmACRPolicyDTO](req)
	if err != nil {
		return nil, err
	}
	return api.RealmACRPolicyFromDTO(body), nil
}

func (c *RealmACRPolicyController) get(w http.ResponseWriter, r *http.Request) {
	policy, err := c.policies.GetPolicy(r.Context(), r.PathValue("realmId"))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.RealmACRPolicyToDTO(policy))
}
