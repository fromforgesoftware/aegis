package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestOIDCController_JWKS(t *testing.T) {
	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().JWKS(mock.Anything, "trading-bot").Return(json.RawMessage(`{"keys":[]}`), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET realm jwks.json returns 200",
			internaltest.NewRESTHandler(aegishttp.NewOIDCController(keys, resolvingRealms(t, "trading-bot"))),
			restest.NewReq(t, context.Background(), http.MethodGet,
				"/realms/trading-bot/.well-known/jwks.json", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOIDCController_Discovery(t *testing.T) {
	// Discovery is built from the request; it never touches the keystore.
	keys := apptest.NewSigningKeyService(t)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET realm openid-configuration returns 200",
			internaltest.NewRESTHandler(aegishttp.NewOIDCController(keys, resolvingRealms(t, "trading-bot"))),
			restest.NewReq(t, context.Background(), http.MethodGet,
				"/realms/trading-bot/.well-known/openid-configuration", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
