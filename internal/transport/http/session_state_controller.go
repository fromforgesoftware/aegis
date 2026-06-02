package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// SessionStateController exposes /api/session-states — tracking and inspecting
// where sessions currently are. Writes happen on the consumer hot path via
// Track; reads are admin/diagnostic.
type SessionStateController struct {
	states app.SessionStateUsecase
}

func NewSessionStateController(states app.SessionStateUsecase) kitrest.Controller {
	return &SessionStateController{states: states}
}

func (c *SessionStateController) Routes(r kitrest.Router) {
	r.Route("/api/session-states", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.track, decodeTrackSessionState, api.SessionStateToDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Track session topology"), openapi.Tags("sessions"), openapi.Errors(400)),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.states, api.SessionStateToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List session states"), openapi.Tags("sessions")),
		))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.states, api.SessionStateToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a session state"), openapi.Tags("sessions"), openapi.Errors(404)),
			))
			r.Post("/touch", http.HandlerFunc(c.touch))
		})
	})
}

func (c *SessionStateController) track(ctx context.Context, s domain.SessionState) (domain.SessionState, error) {
	return c.states.Track(ctx, s)
}

func decodeTrackSessionState(req *http.Request) (domain.SessionState, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.SessionStateDTO](req)
	if err != nil {
		return nil, err
	}
	return api.SessionStateFromDTO(body), nil
}

// touch refreshes a session's last_active so the idle sweeper keeps it.
func (c *SessionStateController) touch(w http.ResponseWriter, r *http.Request) {
	if err := c.states.Touch(r.Context(), r.PathValue("id")); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
