package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestExternalIDPController_Create(t *testing.T) {
	uc := apptest.NewExternalIDPConfigUsecase(t)
	want := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
		internaltest.WithExternalIDPName("google-prod"),
	)
	uc.EXPECT().
		CreateWithSecret(mock.Anything, mock.MatchedBy(internaltest.MatchExternalIDP(want)), "the-secret").
		Return(internaltest.NewExternalIDP(
			internaltest.WithExternalIDPID("idp-1"),
			internaltest.WithExternalIDPRealmID("r"),
			internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
			internaltest.WithExternalIDPName("google-prod"),
		), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/external-idps returns 201",
			internaltest.NewRESTHandler(aegishttp.NewExternalIDPController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/external-idps",
				`{"data":{"type":"externalIdps","attributes":{"realmId":"r","kind":"OAUTH_GOOGLE","name":"google-prod","clientId":"abc","clientSecret":"the-secret","enabled":true}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestExternalIDPController_Get(t *testing.T) {
	uc := apptest.NewExternalIDPConfigUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewExternalIDP(internaltest.WithExternalIDPID("idp-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/external-idps/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewExternalIDPController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/external-idps/idp-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestExternalIDPController_List(t *testing.T) {
	uc := apptest.NewExternalIDPConfigUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.ExternalIDPConfig{
		internaltest.NewExternalIDP(internaltest.WithExternalIDPID("idp-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/external-idps returns 200",
			internaltest.NewRESTHandler(aegishttp.NewExternalIDPController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/external-idps", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestExternalIDPController_Delete(t *testing.T) {
	uc := apptest.NewExternalIDPConfigUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/external-idps/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewExternalIDPController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/external-idps/idp-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
