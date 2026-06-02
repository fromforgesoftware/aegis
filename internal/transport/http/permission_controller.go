package http

import (
	"encoding/json"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// PermissionController exposes /api/permissions as JSON:API for catalog
// inspection and the rare seed-via-API path.
type PermissionController struct {
	permissions app.PermissionUsecase
}

func NewPermissionController(permissions app.PermissionUsecase) kitrest.Controller {
	return &PermissionController{permissions: permissions}
}

func (c *PermissionController) Routes(r kitrest.Router) {
	r.Route("/api/permissions", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCreateHandler(
			c.permissions, api.PermissionFromDTO, api.PermissionToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Register a permission"), openapi.Tags("authz"), openapi.Errors(400, 409)),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.permissions, api.PermissionToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List permissions"), openapi.Tags("authz")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.permissions, api.PermissionToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a permission"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.permissions, repository.DeleteTypeHard,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete a permission"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Get("/implications", http.HandlerFunc(c.listImplications))
			r.Post("/implications", http.HandlerFunc(c.setImplications))
		})
	})
}

type permissionImplicationsDocument struct {
	Data []permissionRef `json:"data"`
}

type permissionRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// listImplications returns the permissions directly implied by this one as a
// JSON:API resource-identifier document.
func (c *PermissionController) listImplications(w http.ResponseWriter, r *http.Request) {
	ids, err := c.permissions.ListImplications(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	refs := make([]permissionRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, permissionRef{Type: string(api.ResourceTypePermission), ID: id})
	}
	writeJSON(w, http.StatusOK, permissionImplicationsDocument{Data: refs})
}

// setImplications overwrites the permission's implication set with the
// resource identifiers in data[].
func (c *PermissionController) setImplications(w http.ResponseWriter, r *http.Request) {
	var body permissionImplicationsDocument
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
	if err := c.permissions.SetImplications(r.Context(), r.PathValue("id"), ids); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
