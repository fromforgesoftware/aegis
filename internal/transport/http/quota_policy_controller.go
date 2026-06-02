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

// QuotaPolicyController exposes /api/quota-policies CRUD plus a
// check operation that reports whether a given current count is under the cap.
type QuotaPolicyController struct {
	quota app.QuotaUsecase
}

func NewQuotaPolicyController(quota app.QuotaUsecase) kitrest.Controller {
	return &QuotaPolicyController{quota: quota}
}

func (c *QuotaPolicyController) Routes(r kitrest.Router) {
	r.Route("/api/quota-policies", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.set, decodeSetQuotaPolicy, api.QuotaPolicyToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Set a realm quota policy"), openapi.Tags("quotas"), openapi.Errors(400, 409)),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.quota, api.QuotaPolicyToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List realm quota policies"), openapi.Tags("quotas")),
		))
		r.Post("/check", kitrest.NewJsonApiCommandHandler(
			c.check, decodeQuotaCheck, quotaCheckToDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Check a realm quota"), openapi.Tags("quotas"), openapi.Errors(400)),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.quota, api.QuotaPolicyToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a realm quota policy"), openapi.Tags("quotas"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.quota, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete a realm quota policy"), openapi.Tags("quotas"), openapi.Errors(404)),
			))
		})
	})
}

func (c *QuotaPolicyController) set(ctx context.Context, p domain.QuotaPolicy) (domain.QuotaPolicy, error) {
	return c.quota.SetPolicy(ctx, p)
}

func decodeSetQuotaPolicy(req *http.Request) (domain.QuotaPolicy, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.QuotaPolicyDTO](req)
	if err != nil {
		return nil, err
	}
	return api.QuotaPolicyFromDTO(body), nil
}

func (c *QuotaPolicyController) check(ctx context.Context, in api.QuotaCheckRequestDTO) (*api.QuotaCheckDTO, error) {
	allowed, err := c.quota.Allow(ctx, in.RRealmID, in.RResourceType, in.RCurrent)
	if err != nil {
		return nil, err
	}
	return api.QuotaCheckToDTO(allowed), nil
}

func decodeQuotaCheck(req *http.Request) (api.QuotaCheckRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.QuotaCheckRequestDTO](req)
	if err != nil {
		return api.QuotaCheckRequestDTO{}, err
	}
	return *body, nil
}

func quotaCheckToDTO(dto *api.QuotaCheckDTO) *api.QuotaCheckDTO { return dto }
