package http_test

import (
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestServiceAccountController_Create(t *testing.T) {
	uc := apptest.NewServiceAccountUsecase(t)
	uc.EXPECT().Create(mock.Anything, "r", "ci-bot", []string{"audit:read"}).
		Return(app.ServiceAccountCredentials{
			ServiceAccount: domain.NewServiceAccount("r", "ci-bot", "client-x", domain.WithServiceAccountID("acc-svc")),
			ClientID:       "client-x",
			ClientSecret:   "raw-secret",
		}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/service-accounts returns 201 with secret",
			internaltest.NewRESTHandler(aegishttp.NewServiceAccountController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/service-accounts",
				`{"data":{"type":"service-accounts","attributes":{"realmId":"r","name":"ci-bot","scopes":["audit:read"]}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestServiceAccountController_Token(t *testing.T) {
	uc := apptest.NewServiceAccountUsecase(t)
	uc.EXPECT().IssueToken(mock.Anything, "r", mock.Anything, "client-x", "raw-secret").
		Return(app.TokenResponse{AccessToken: "token-abc", TokenType: "Bearer", ExpiresIn: 3600}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/service-accounts/token returns 200",
			internaltest.NewRESTHandler(aegishttp.NewServiceAccountController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/service-accounts/token",
				`{"data":{"type":"serviceAccountTokens","attributes":{"realmId":"r","clientId":"client-x","clientSecret":"raw-secret"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
