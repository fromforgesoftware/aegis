package http_test

import (
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestFederatedSignInController_ResolveExistingAccount(t *testing.T) {
	broker := apptest.NewIdentityBrokerUsecase(t)
	broker.EXPECT().ResolveAccount(mock.Anything, app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "the-token",
	}).Return(app.ResolveAccountResult{
		Account: internaltest.NewAccount(internaltest.WithAccountID("acc-1"), internaltest.WithAccountEmail("a@b.com")),
		Created: false,
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/auth/federate returns 200",
			internaltest.NewRESTHandler(aegishttp.NewFederatedSignInController(broker, permissiveRL())),
			jsonapiReq(t, http.MethodPost, "/api/auth/federate",
				`{"data":{"type":"federatedSignIns","attributes":{"realmId":"r","idpName":"google-prod","token":"the-token"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestFederatedSignInController_LinkRequired(t *testing.T) {
	broker := apptest.NewIdentityBrokerUsecase(t)
	broker.EXPECT().ResolveAccount(mock.Anything, app.ResolveAccountInput{
		RealmID: "r", IDPName: "github-prod", RawToken: "ghp_TOKEN",
	}).Return(app.ResolveAccountResult{
		Account:      internaltest.NewAccount(internaltest.WithAccountID("acc-existing")),
		LinkRequired: true,
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/auth/federate surfaces linkRequired=true",
			internaltest.NewRESTHandler(aegishttp.NewFederatedSignInController(broker, permissiveRL())),
			jsonapiReq(t, http.MethodPost, "/api/auth/federate",
				`{"data":{"type":"federatedSignIns","attributes":{"realmId":"r","idpName":"github-prod","token":"ghp_TOKEN"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
