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

// GroupController exposes /api/groups as JSON:API plus a sub-resource
// /api/groups/{id}/members for managing the group's account membership.
type GroupController struct {
	groups app.GroupUsecase
}

func NewGroupController(groups app.GroupUsecase) kitrest.Controller {
	return &GroupController{groups: groups}
}

type createGroupCommand struct {
	Group   domain.Group
	Members []string
}

func (c *GroupController) Routes(r kitrest.Router) {
	r.Route("/api/groups", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.create, decodeCreateGroup, api.GroupToDTO,
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Create a group"),
				openapi.Description("Optional members attribute seeds the group's account membership atomically."),
				openapi.Tags("authz"), openapi.Errors(400, 409),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.groups, api.GroupToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List groups"), openapi.Tags("authz")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.groups, api.GroupToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a group"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Delete("", kitrest.NewJsonApiDeleteHandler(
				c.groups, repository.DeleteTypeSoft,
				kitrest.HandlerWithOpenAPI(openapi.Summary("Delete a group"), openapi.Tags("authz"), openapi.Errors(404)),
			))
			r.Get("/members", http.HandlerFunc(c.listMembers))
			r.Post("/members", http.HandlerFunc(c.setMembers))
		})
	})
}

func (c *GroupController) create(ctx context.Context, cmd createGroupCommand) (domain.Group, error) {
	return c.groups.Create(ctx, cmd.Group, cmd.Members)
}

// decodeCreateGroup parses the JSON:API group body plus the synthetic
// "members" attribute (an account-id array) that seeds the initial membership
// without forcing the caller to learn relationships first.
func decodeCreateGroup(req *http.Request) (createGroupCommand, error) {
	type groupWithMembers struct {
		api.GroupDTO
		RMembers []string `jsonapi:"attr,members,omitempty"`
	}
	body, err := kitrest.UnmarshalPayloadFromRequest[*groupWithMembers](req)
	if err != nil {
		return createGroupCommand{}, err
	}
	return createGroupCommand{
		Group:   api.GroupFromDTO(&body.GroupDTO),
		Members: body.RMembers,
	}, nil
}

type groupMembersResponse struct {
	Data []groupMemberRef `json:"data"`
}

type groupMembersRequest struct {
	Data []groupMemberRef `json:"data"`
}

type groupMemberRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// listMembers returns the account ids in the group as a JSON:API relationship
// document.
func (c *GroupController) listMembers(w http.ResponseWriter, r *http.Request) {
	ids, err := c.groups.ListMembers(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	refs := make([]groupMemberRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, groupMemberRef{Type: "accounts", ID: id})
	}
	writeJSON(w, http.StatusOK, groupMembersResponse{Data: refs})
}

// setMembers replaces the group's membership with the resource identifiers in
// data[]. Idempotent overwrite — the group's prior members are gone after a
// successful call.
func (c *GroupController) setMembers(w http.ResponseWriter, r *http.Request) {
	var body groupMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	ids := make([]string, 0, len(body.Data))
	for _, ref := range body.Data {
		if ref.Type != "accounts" {
			writeJSONError(w, apierrors.InvalidArgument("relationship type must be accounts"))
			return
		}
		ids = append(ids, ref.ID)
	}
	if err := c.groups.SetMembers(r.Context(), r.PathValue("id"), ids); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
