package http

import (
	"context"
	"encoding/json"
	"net/http"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// InvitationController exposes /api/invitations: admins create invites (emailed
// via NotificationSender) and invitees accept them to gain the pre-assigned
// role on the resource.
type InvitationController struct {
	invitations app.InvitationUsecase
}

func NewInvitationController(invitations app.InvitationUsecase) kitrest.Controller {
	return &InvitationController{invitations: invitations}
}

func (c *InvitationController) Routes(r kitrest.Router) {
	r.Route("/api/invitations", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.create, decodeCreateInvitation, api.InvitationToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Create an invitation"), openapi.Tags("admin"), openapi.Errors(400, 409)),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.invitations, api.InvitationToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List invitations"), openapi.Tags("admin")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.invitations, api.InvitationToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get an invitation"), openapi.Tags("admin"), openapi.Errors(404)),
			))
		})
		r.Post("/accept", http.HandlerFunc(c.accept))
	})
}

func (c *InvitationController) create(ctx context.Context, inv domain.Invitation) (domain.Invitation, error) {
	return c.invitations.Create(ctx, inv)
}

func decodeCreateInvitation(req *http.Request) (domain.Invitation, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.InvitationDTO](req)
	if err != nil {
		return nil, err
	}
	return api.InvitationFromDTO(body), nil
}

// accept consumes a token (delivered by email) and binds the account. Plain
// JSON: the token is a bearer secret, not a JSON:API resource.
func (c *InvitationController) accept(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token     string `json:"token"`
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if err := c.invitations.Accept(r.Context(), body.Token, body.AccountID); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
