package http_test

import (
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestMagicLinkController_Request(t *testing.T) {
	uc := apptest.NewMagicLinkUsecase(t)
	uc.EXPECT().RequestMagicLink(mock.Anything, "realm-1", "user@x.com").Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/magic-links returns 202",
			internaltest.NewRESTHandler(aegishttp.NewMagicLinkController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/magic-links",
				`{"data":{"type":"magicLinks","attributes":{"realmId":"realm-1","email":"user@x.com"}}}`),
			restest.AssertResponseStatus(http.StatusAccepted),
		),
	).Exec(t)
}

func TestMagicLinkController_Redeem(t *testing.T) {
	uc := apptest.NewMagicLinkUsecase(t)
	uc.EXPECT().RedeemMagicLink(mock.Anything, "raw-token").
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
			internaltest.WithAccountRealmID("realm-1"), internaltest.WithAccountEmail("user@x.com")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/magic-links/redeem returns 200",
			internaltest.NewRESTHandler(aegishttp.NewMagicLinkController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/magic-links/redeem",
				`{"data":{"type":"magicLinkSessions","attributes":{"token":"raw-token"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
