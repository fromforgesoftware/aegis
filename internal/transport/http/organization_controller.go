package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/auth"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/fields"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// OrganizationController exposes /api/organizations as JSON:API plus a
// /api/organizations/{id}/members sub-resource backed by authz bindings.
type OrganizationController struct {
	orgs app.OrganizationUsecase
}

func NewOrganizationController(orgs app.OrganizationUsecase) kitrest.Controller {
	return &OrganizationController{orgs: orgs}
}

func (c *OrganizationController) Routes(r kitrest.Router) {
	r.Route("/api/organizations", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.create, decodeOrganization, api.OrganizationToDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Create an organization"),
				openapi.Description("The caller becomes the organization owner unless an owner relationship is supplied."),
				openapi.Tags("tenancy"), openapi.Errors(400, 409),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.orgs, api.OrganizationToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List organizations"), openapi.Tags("tenancy")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.orgs, api.OrganizationToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get an organization"), openapi.Tags("tenancy"), openapi.Errors(404)),
			))
			r.Patch("", http.HandlerFunc(c.patch))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.orgs, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete an organization"), openapi.Tags("tenancy"), openapi.Errors(404)),
			))
			r.Post("/activate", http.HandlerFunc(c.activate))
			r.Get("/members", http.HandlerFunc(c.listMembers))
			r.Post("/members", http.HandlerFunc(c.addMember))
			r.Delete("/members/{accountId}", http.HandlerFunc(c.removeMember))
		})
	})
	r.Get("/api/me/organizations", http.HandlerFunc(c.listMine))
}

func patchOrganization(id string, dto *api.OrganizationDTO) []repository.PatchOption {
	opts := []repository.PatchOption{
		repository.PatchSearchOpts(search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, id))),
	}
	if dto.RName != "" {
		opts = append(opts, repository.PatchField(fields.Name, dto.RName))
	}
	if dto.RSlug != "" {
		opts = append(opts, repository.PatchField(fields.Slug, dto.RSlug))
	}
	if dto.RStatus != "" {
		opts = append(opts, repository.PatchField(fields.Status, dto.RStatus))
	}
	return opts
}

func (c *OrganizationController) patch(w http.ResponseWriter, r *http.Request) {
	dto, err := kitrest.UnmarshalPayloadFromRequest[*api.OrganizationDTO](r)
	if err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	patched, err := c.orgs.Patch(r.Context(), patchOrganization(r.PathValue("id"), dto)...)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	if len(patched) == 0 {
		writeJSONError(w, apierrors.NotFound("organization", r.PathValue("id")))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalPayload(w, api.OrganizationToDTO(patched[0]))
}

func (c *OrganizationController) activate(w http.ResponseWriter, r *http.Request) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	if err := c.orgs.Activate(r.Context(), tok.Claims().Subject(), r.PathValue("id")); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *OrganizationController) listMine(w http.ResponseWriter, r *http.Request) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	orgs, err := c.orgs.ListForAccount(r.Context(), tok.Claims().Subject())
	if err != nil {
		writeJSONError(w, err)
		return
	}
	list := resource.ListResponseToDTO(api.OrganizationToDTO)(resource.NewListResponse(orgs, len(orgs)))
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

func (c *OrganizationController) create(ctx context.Context, org domain.Organization) (domain.Organization, error) {
	if org.Owner() == nil {
		if tok := auth.TokenFromCtx(ctx); tok != nil {
			if sub := tok.Claims().Subject(); sub != "" {
				org = domain.NewOrganization(org.Realm().ID(), org.Name(), org.Slug(),
					domain.WithOrganizationOwnerID(sub),
					domain.WithOrganizationStatus(org.Status()),
					domain.WithOrganizationSettings(org.Settings()),
				)
			}
		}
	}
	return c.orgs.Create(ctx, org)
}

func decodeOrganization(req *http.Request) (domain.Organization, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.OrganizationDTO](req)
	if err != nil {
		return nil, err
	}
	return api.OrganizationFromDTO(body), nil
}

type addOrgMemberRequest struct {
	AccountID string `json:"accountId"`
	Role      string `json:"role"`
}

func (c *OrganizationController) listMembers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	members, err := c.orgs.ListMembers(r.Context(), orgID)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	list := resource.ListResponseToDTO(func(b domain.Binding) *api.MembershipDTO {
		return api.MembershipToDTO(b, orgID)
	})(resource.NewListResponse(members, len(members)))
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

func (c *OrganizationController) addMember(w http.ResponseWriter, r *http.Request) {
	var body addOrgMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if body.AccountID == "" || body.Role == "" {
		writeJSONError(w, apierrors.InvalidArgument("accountId and role are required"))
		return
	}
	if err := c.orgs.AddMember(r.Context(), r.PathValue("id"), body.AccountID, body.Role); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *OrganizationController) removeMember(w http.ResponseWriter, r *http.Request) {
	if err := c.orgs.RemoveMember(r.Context(), r.PathValue("id"), r.PathValue("accountId")); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
