package http_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func sessionCookieReq(req *http.Request, value string) *http.Request {
	req.AddCookie(&http.Cookie{Name: "aegis_session", Value: value})
	return req
}

func TestOAuthController_AuthorizeNoSessionRedirectsToLogin(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /authorize without a session bounces to hosted login (302)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			restest.NewReq(t, context.Background(), http.MethodGet,
				"/realms/r/authorize?response_type=code&client_id=web&redirect_uri=https://app/cb", http.NoBody),
			restest.AssertResponseStatus(http.StatusFound),
		),
	).Exec(t)
}

func TestOAuthController_AuthorizeUnknownRealmRejected(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	realms := apptest.NewRealmUsecase(t)
	realms.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("realm", "nope"))

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /authorize for an unknown realm is rejected (404)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, realms)),
			restest.NewReq(t, context.Background(), http.MethodGet,
				"/realms/nope/authorize?response_type=code&client_id=web&redirect_uri=https://app/cb", http.NoBody),
			restest.AssertResponseStatus(http.StatusNotFound),
		),
	).Exec(t)
}

func TestOAuthController_AuthorizeWithSessionRedirectsWithCode(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().ResolveSession(mock.Anything, "sess-1").
		Return(internaltest.NewSession(internaltest.WithSessionID("sess-1"), internaltest.WithSessionRealmID("r"), internaltest.WithSessionAccountID("acc-1")), nil)
	uc.EXPECT().Authorize(mock.Anything, mock.MatchedBy(func(in app.AuthorizeInput) bool {
		return in.RealmID == "r" && in.ClientID == "web" && in.AccountID == "acc-1" &&
			in.RedirectURI == "https://app/cb" && in.CodeChallenge == "ch"
	})).Return(app.AuthorizeResult{Code: "the-code", RedirectURI: "https://app/cb", State: "s-1"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /authorize with a valid session redirects to the client with code (302)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			sessionCookieReq(restest.NewReq(t, context.Background(), http.MethodGet,
				"/realms/r/authorize?response_type=code&client_id=web&redirect_uri=https://app/cb&state=s-1&code_challenge=ch&code_challenge_method=S256", http.NoBody), "sess-1"),
			restest.AssertResponseStatus(http.StatusFound),
		),
	).Exec(t)
}

func TestOAuthController_TokenAuthorizationCode(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().ExchangeCode(mock.Anything, mock.MatchedBy(func(in app.CodeExchangeInput) bool {
		return in.RealmID == "r" && in.ClientID == "web" && in.Code == "the-code" &&
			in.RedirectURI == "https://app/cb" && in.CodeVerifier == "verifier-123"
	})).Return(app.TokenResponse{
		AccessToken: "at", TokenType: "Bearer", ExpiresIn: 3600, IDToken: "idt", Scope: "openid",
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /token authorization_code grant returns the token (200)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			tokenReq(t, "/realms/r/token", url.Values{
				"grant_type":    {"authorization_code"},
				"client_id":     {"web"},
				"code":          {"the-code"},
				"redirect_uri":  {"https://app/cb"},
				"code_verifier": {"verifier-123"},
			}),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOAuthController_TokenClientCredentials(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().ClientCredentials(mock.Anything, mock.MatchedBy(func(in app.ClientCredentialsInput) bool {
		return in.RealmID == "r" && in.ClientID == "svc" && in.ClientSecret == "s3cret" &&
			len(in.Scopes) == 1 && in.Scopes[0] == "trade.place"
	})).Return(app.TokenResponse{AccessToken: "at", TokenType: "Bearer", ExpiresIn: 3600, Scope: "trade.place"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /token client_credentials grant returns the token (200)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			tokenReq(t, "/realms/r/token", url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {"svc"},
				"client_secret": {"s3cret"},
				"scope":         {"trade.place"},
			}),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOAuthController_TokenRefresh(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Refresh(mock.Anything, mock.MatchedBy(func(in app.RefreshInput) bool {
		return in.RealmID == "r" && in.ClientID == "web" && in.RefreshToken == "the-refresh"
	})).Return(app.TokenResponse{
		AccessToken: "at", TokenType: "Bearer", ExpiresIn: 3600, RefreshToken: "new-refresh", Scope: "openid",
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /token refresh_token grant rotates the token (200)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			tokenReq(t, "/realms/r/token", url.Values{
				"grant_type":    {"refresh_token"},
				"client_id":     {"web"},
				"refresh_token": {"the-refresh"},
			}),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOAuthController_TokenUnsupportedGrant(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /token with an unsupported grant_type is 400",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			tokenReq(t, "/realms/r/token", url.Values{"grant_type": {"password"}}),
			restest.AssertResponseStatus(http.StatusBadRequest),
		),
	).Exec(t)
}

func TestOAuthController_Introspect(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Introspect(mock.Anything, mock.MatchedBy(func(in app.IntrospectInput) bool {
		return in.RealmID == "r" && in.ClientID == "web" && in.Token == "the-token"
	})).Return(app.IntrospectionResult{Active: true, TokenType: "Bearer", Subject: "acc-1", Scope: "openid"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /introspect returns the token state (200)",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			tokenReq(t, "/realms/r/introspect", url.Values{
				"client_id": {"web"},
				"token":     {"the-token"},
			}),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestOAuthController_Revoke(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Revoke(mock.Anything, mock.MatchedBy(func(in app.RevokeInput) bool {
		return in.RealmID == "r" && in.ClientID == "web" && in.Token == "the-token"
	})).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /revoke acknowledges with 200",
			internaltest.NewRESTHandler(aegishttp.NewOAuthController(uc, resolvingRealms(t, "r"))),
			tokenReq(t, "/realms/r/revoke", url.Values{
				"client_id": {"web"},
				"token":     {"the-token"},
			}),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func tokenReq(t *testing.T, target string, form url.Values) *http.Request {
	req := restest.NewReq(t, context.Background(), http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}
