package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// RoleController exposes /api/roles as JSON:API plus a sub-resource
// /api/roles/{id}/permissions for managing the role's permission set.
type RoleController struct {
	roles app.RoleUsecase
}

func NewRoleController(roles app.RoleUsecase) kitrest.Controller {
	return &RoleController{roles: roles}
}

type createRoleCommand struct {
	Role        domain.Role
	Permissions []string
}

func (c *RoleController) Routes(r kitrest.Router) {
	r.Route("/api/roles", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.create, decodeCreateRole, api.RoleToDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Create a custom role"),
				openapi.Description("Optional relationships.permissions seeds the role's permission set atomically."),
				openapi.Tags("authz"), openapi.Errors(400, 409),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.roles, api.RoleToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List roles"), openapi.Tags("authz")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.roles, api.RoleToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a role"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.roles, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete a role"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Get("/permissions", http.HandlerFunc(c.listPermissions))
			r.Post("/permissions", http.HandlerFunc(c.setPermissions))
			r.Get("/composition", http.HandlerFunc(c.listComposition))
			r.Post("/composition", http.HandlerFunc(c.setComposition))
		})
	})
}

func (c *RoleController) create(ctx context.Context, cmd createRoleCommand) (domain.Role, error) {
	return c.roles.Create(ctx, cmd.Role, cmd.Permissions)
}

// decodeCreateRole parses the JSON:API role body plus the synthetic
// "permissions" attribute (a slug array) that selects the initial permission
// set without forcing the caller to learn relationships first.
func decodeCreateRole(req *http.Request) (createRoleCommand, error) {
	type roleWithPerms struct {
		api.RoleDTO
		RPermissions []string `jsonapi:"attr,permissions,omitempty"`
	}
	body, err := kitrest.UnmarshalPayloadFromRequest[*roleWithPerms](req)
	if err != nil {
		return createRoleCommand{}, err
	}
	return createRoleCommand{
		Role:        api.RoleFromDTO(&body.RoleDTO),
		Permissions: body.RPermissions,
	}, nil
}

type rolePermissionsResponse struct {
	Data []rolePermissionRef `json:"data"`
}

type rolePermissionsRequest struct {
	Data []rolePermissionRef `json:"data"`
}

type rolePermissionRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// listPermissions returns the slugs attached to the role as a JSON:API
// relationship document.
func (c *RoleController) listPermissions(w http.ResponseWriter, r *http.Request) {
	ids, err := c.roles.ListPermissions(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	refs := make([]rolePermissionRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, rolePermissionRef{Type: string(api.ResourceTypePermission), ID: id})
	}
	writeJSON(w, http.StatusOK, rolePermissionsResponse{Data: refs})
}

// setPermissions replaces the role's permission set with the resource
// identifiers in data[]. Idempotent overwrite — the role's prior permissions
// are gone after a successful call.
func (c *RoleController) setPermissions(w http.ResponseWriter, r *http.Request) {
	var body rolePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	ids := make([]string, 0, len(body.Data))
	for _, ref := range body.Data {
		if ref.Type != string(api.ResourceTypePermission) {
			writeJSONError(w, apierrors.InvalidArgument("relationship type must be permissions"))
			return
		}
		ids = append(ids, ref.ID)
	}
	if err := c.roles.SetPermissions(r.Context(), r.PathValue("id"), ids); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type roleCompositionDocument struct {
	Data []roleComponentDTO `json:"data"`
}

type roleComponentDTO struct {
	ComponentRoleID string `json:"componentRoleId"`
	Operator        string `json:"operator"`
	Ordinal         int    `json:"ordinal"`
}

// listComposition returns the role's ordered component list.
func (c *RoleController) listComposition(w http.ResponseWriter, r *http.Request) {
	components, err := c.roles.ListComposition(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	out := make([]roleComponentDTO, 0, len(components))
	for _, comp := range components {
		out = append(out, roleComponentDTO{
			ComponentRoleID: comp.ComponentRoleID,
			Operator:        string(comp.Operator),
			Ordinal:         comp.Ordinal,
		})
	}
	writeJSON(w, http.StatusOK, roleCompositionDocument{Data: out})
}

// setComposition overwrites the role's composition with the ordered components
// in data[].
func (c *RoleController) setComposition(w http.ResponseWriter, r *http.Request) {
	var body roleCompositionDocument
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	components := make([]domain.RoleComponent, 0, len(body.Data))
	for _, comp := range body.Data {
		components = append(components, domain.RoleComponent{
			ComponentRoleID: comp.ComponentRoleID,
			Operator:        domain.CompositionOperator(comp.Operator),
			Ordinal:         comp.Ordinal,
		})
	}
	if err := c.roles.SetComposition(r.Context(), r.PathValue("id"), components); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
