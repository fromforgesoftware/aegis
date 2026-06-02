package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// ServiceAccountController exposes machine identities at /api/service-accounts:
// create (returns credentials once), list/get, delete, and a client_credentials
// token endpoint that mints account-subject'd access tokens.
type ServiceAccountController struct {
	svc app.ServiceAccountUsecase
}

func NewServiceAccountController(svc app.ServiceAccountUsecase) kitrest.Controller {
	return &ServiceAccountController{svc: svc}
}

func (c *ServiceAccountController) Routes(r kitrest.Router) {
	r.Route("/api/service-accounts", func(r kitrest.Router) {
		r.Post("", kitrest.NewJsonApiCommandHandler(
			c.create, decodeServiceAccountCreate, identityServiceAccountCredentialsDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusCreated),
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Create a service account"),
				openapi.Description("Creates a SERVICE account with machine credentials; clientSecret is returned once."),
				openapi.Tags("service-accounts"), openapi.Errors(400),
			),
		))
		r.Get("", kitrest.NewJsonApiListHandler(
			c.svc, api.ServiceAccountToDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("List service accounts"), openapi.Tags("service-accounts")),
		))
		// Static /token before the /{id} param route so chi matches it first.
		r.Post("/token", http.HandlerFunc(c.token))
		r.Route("/{id}", func(r kitrest.Router) {
			r.Get("", kitrest.NewJsonApiGetHandler(
				c.svc, api.ServiceAccountToDTO, []query.ParseOpt{},
				kitrest.HandlerWithOpenAPI(openapi.Summary("Get a service account"), openapi.Tags("service-accounts"), openapi.Errors(404)),
			))
			r.Delete("", http.HandlerFunc(c.delete))
		})
	})
}

func (c *ServiceAccountController) create(ctx context.Context, in api.ServiceAccountCreateDTO) (*api.ServiceAccountCredentialsDTO, error) {
	creds, err := c.svc.Create(ctx, in.RRealmID, in.RName, in.RScopes)
	if err != nil {
		return nil, err
	}
	return api.ServiceAccountCredentialsToDTO(creds.ServiceAccount, creds.ClientID, creds.ClientSecret), nil
}

// token mints a client_credentials access token. It's a raw handler so the
// issuer can be derived from the request host, matching the OAuth endpoints.
func (c *ServiceAccountController) token(w http.ResponseWriter, r *http.Request) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.ServiceAccountTokenRequestDTO](r)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	issuer := requestScheme(r) + "://" + requestHost(r) + "/realms/" + body.RRealmID
	resp, err := c.svc.IssueToken(r.Context(), body.RRealmID, issuer, body.RClientID, body.RClientSecret)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.ServiceAccountTokenToDTO(resp.AccessToken, resp.TokenType, resp.ExpiresIn))
}

func (c *ServiceAccountController) delete(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeServiceAccountCreate(req *http.Request) (api.ServiceAccountCreateDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.ServiceAccountCreateDTO](req)
	if err != nil {
		return api.ServiceAccountCreateDTO{}, err
	}
	return *body, nil
}

func identityServiceAccountCredentialsDTO(dto *api.ServiceAccountCredentialsDTO) *api.ServiceAccountCredentialsDTO {
	return dto
}
