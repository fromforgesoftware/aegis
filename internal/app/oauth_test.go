package app_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

type stubActiveOrgResolver struct{}

func (stubActiveOrgResolver) ActiveOrg(context.Context, string) (string, string, error) {
	return "", "", nil
}

func newOAuth(t *testing.T) (
	*apptest.ClientRepository,
	*apptest.AccountRepository,
	*apptest.AuthorizationCodeRepository,
	*apptest.SessionRepository,
	*apptest.SigningKeyService,
	app.OAuthUsecase,
) {
	clients := apptest.NewClientRepository(t)
	accounts := apptest.NewAccountRepository(t)
	codes := apptest.NewAuthorizationCodeRepository(t)
	sessions := apptest.NewSessionRepository(t)
	keys := apptest.NewSigningKeyService(t)
	refresh := apptest.NewRefreshTokenRepository(t)
	uc := app.NewOAuthUsecase(clients, accounts, codes, refresh, sessions, app.NewTokenIssuer(keys), stubActiveOrgResolver{})
	return clients, accounts, codes, sessions, keys, uc
}

// newOAuthRefresh wires a usecase exposing the refresh-token mocks the
// rotation and code-exchange-with-refresh paths drive.
func newOAuthRefresh(t *testing.T) (
	*apptest.ClientRepository,
	*apptest.AccountRepository,
	*apptest.AuthorizationCodeRepository,
	*apptest.RefreshTokenRepository,
	*apptest.SessionRepository,
	*apptest.SigningKeyService,
	app.OAuthUsecase,
) {
	clients := apptest.NewClientRepository(t)
	accounts := apptest.NewAccountRepository(t)
	codes := apptest.NewAuthorizationCodeRepository(t)
	sessions := apptest.NewSessionRepository(t)
	keys := apptest.NewSigningKeyService(t)
	refresh := apptest.NewRefreshTokenRepository(t)
	uc := app.NewOAuthUsecase(clients, accounts, codes, refresh, sessions, app.NewTokenIssuer(keys), stubActiveOrgResolver{})
	return clients, accounts, codes, refresh, sessions, keys, uc
}

func confidentialWeb(secretHash string) domain.Client {
	return domain.NewClient("r", "web", domain.ClientTypeConfidential, "Web",
		domain.WithClientRedirectURIs([]string{"https://app/cb"}),
		domain.WithClientSecretHash(secretHash))
}

func serviceClient(secretHash string) domain.Client {
	return domain.NewClient("r", "svc", domain.ClientTypeConfidential, "Service",
		domain.WithClientGrantTypes([]string{"client_credentials"}),
		domain.WithClientScopes([]string{"trade.place", "portfolio.view"}),
		domain.WithClientSecretHash(secretHash))
}

func publicWeb() domain.Client {
	return domain.NewClient("r", "web", domain.ClientTypePublic, "Web",
		domain.WithClientRedirectURIs([]string{"https://app/cb"}))
}

func TestAuthorize_Success(t *testing.T) {
	clients, _, codes, _, _, uc := newOAuth(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	codes.EXPECT().Create(mock.Anything, mock.MatchedBy(func(c domain.AuthorizationCode) bool {
		return c.Code != "" && c.RealmID == "r" && c.ClientID == "web" &&
			c.AccountID == "acc-1" && c.RedirectURI == "https://app/cb" && c.PKCEChallenge == "ch"
	})).Return(nil)

	res, err := uc.Authorize(context.Background(), app.AuthorizeInput{
		RealmID: "r", ClientID: "web", AccountID: "acc-1", RedirectURI: "https://app/cb",
		Scopes: []string{"openid"}, State: "s-1", CodeChallenge: "ch", CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Code)
	assert.Equal(t, "s-1", res.State)
	assert.Equal(t, "https://app/cb", res.RedirectURI)
}

func TestAuthorize_RedirectNotRegistered(t *testing.T) {
	clients, _, _, _, _, uc := newOAuth(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	// No code is created when the redirect_uri isn't registered.
	_, err := uc.Authorize(context.Background(), app.AuthorizeInput{
		RealmID: "r", ClientID: "web", AccountID: "acc-1", RedirectURI: "https://evil/cb",
		CodeChallenge: "ch", CodeChallengeMethod: "S256",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestAuthorize_PKCERequiredMissing(t *testing.T) {
	clients, _, _, _, _, uc := newOAuth(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil) // PKCE defaults required
	_, err := uc.Authorize(context.Background(), app.AuthorizeInput{
		RealmID: "r", ClientID: "web", AccountID: "acc-1", RedirectURI: "https://app/cb",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestExchangeCode_SuccessWithPKCEAndIDToken(t *testing.T) {
	clients, accounts, codes, _, keys, uc := newOAuth(t)
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)

	codes.EXPECT().Consume(mock.Anything, "the-code", mock.Anything).Return(domain.AuthorizationCode{
		Code: "the-code", RealmID: "r", ClientID: "web", AccountID: "acc-1",
		RedirectURI: "https://app/cb", Scopes: []string{"openid"},
		PKCEChallenge: cryptox.PKCEChallengeS256("verifier-123"),
	}, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"), internaltest.WithAccountEmail("a@b.com")), nil)

	resp, err := uc.ExchangeCode(context.Background(), app.CodeExchangeInput{
		RealmID: "r", Issuer: "https://auth/realms/r", ClientID: "web",
		Code: "the-code", RedirectURI: "https://app/cb", CodeVerifier: "verifier-123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.IDToken, "openid scope yields an id_token")
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, "openid", resp.Scope)
}

func TestExchangeCode_IssuesRefreshToken(t *testing.T) {
	clients, accounts, codes, refresh, _, keys, uc := newOAuthRefresh(t)
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)

	codes.EXPECT().Consume(mock.Anything, "the-code", mock.Anything).Return(domain.AuthorizationCode{
		Code: "the-code", RealmID: "r", ClientID: "web", AccountID: "acc-1", SessionID: "sess-1",
		RedirectURI: "https://app/cb", Scopes: []string{"openid"},
		PKCEChallenge: cryptox.PKCEChallengeS256("verifier-123"),
	}, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(
		domain.NewClient("r", "web", domain.ClientTypePublic, "Web",
			domain.WithClientRedirectURIs([]string{"https://app/cb"}),
			domain.WithClientGrantTypes([]string{"authorization_code", "refresh_token"})), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"), internaltest.WithAccountEmail("a@b.com")), nil)
	refresh.EXPECT().Create(mock.Anything, mock.MatchedBy(func(rt domain.RefreshToken) bool {
		return rt.SessionID == "sess-1" && rt.ClientID == "web" && rt.RotatedFrom == "" && rt.TokenHash != ""
	})).Return(nil)

	resp, err := uc.ExchangeCode(context.Background(), app.CodeExchangeInput{
		RealmID: "r", Issuer: "https://auth/realms/r", ClientID: "web",
		Code: "the-code", RedirectURI: "https://app/cb", CodeVerifier: "verifier-123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken, "refresh_token grant + session yields a refresh token")
}

func TestExchangeCode_PKCEMismatch(t *testing.T) {
	clients, _, codes, _, _, uc := newOAuth(t)
	codes.EXPECT().Consume(mock.Anything, "c", mock.Anything).Return(domain.AuthorizationCode{
		Code: "c", RealmID: "r", ClientID: "web", RedirectURI: "https://app/cb",
		PKCEChallenge: cryptox.PKCEChallengeS256("right"),
	}, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)

	_, err := uc.ExchangeCode(context.Background(), app.CodeExchangeInput{
		RealmID: "r", ClientID: "web", Code: "c", RedirectURI: "https://app/cb", CodeVerifier: "wrong",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestExchangeCode_RedirectMismatch(t *testing.T) {
	_, _, codes, _, _, uc := newOAuth(t)
	codes.EXPECT().Consume(mock.Anything, "c", mock.Anything).Return(domain.AuthorizationCode{
		Code: "c", RealmID: "r", ClientID: "web", RedirectURI: "https://app/cb",
	}, nil)

	_, err := uc.ExchangeCode(context.Background(), app.CodeExchangeInput{
		RealmID: "r", ClientID: "web", Code: "c", RedirectURI: "https://other/cb",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestExchangeCode_ConfidentialBadSecret(t *testing.T) {
	clients, _, codes, _, _, uc := newOAuth(t)
	codes.EXPECT().Consume(mock.Anything, "c", mock.Anything).Return(domain.AuthorizationCode{
		Code: "c", RealmID: "r", ClientID: "web", RedirectURI: "https://app/cb",
	}, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(confidentialWeb("a-different-hash"), nil)

	_, err := uc.ExchangeCode(context.Background(), app.CodeExchangeInput{
		RealmID: "r", ClientID: "web", Code: "c", RedirectURI: "https://app/cb", ClientSecret: "wrong",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "want UNAUTHENTICATED, got %v", err)
}

func hashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TestClientCredentials_Success(t *testing.T) {
	clients, _, _, _, keys, uc := newOAuth(t)
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(serviceClient(hashSecret("s3cret")), nil)

	resp, err := uc.ClientCredentials(context.Background(), app.ClientCredentialsInput{
		RealmID: "r", Issuer: "https://auth/realms/r", ClientID: "svc",
		ClientSecret: "s3cret", Scopes: []string{"trade.place"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.Empty(t, resp.IDToken, "client_credentials yields no id_token")
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, "trade.place", resp.Scope)
}

func TestClientCredentials_BadSecret(t *testing.T) {
	clients, _, _, _, _, uc := newOAuth(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(serviceClient(hashSecret("right")), nil)

	_, err := uc.ClientCredentials(context.Background(), app.ClientCredentialsInput{
		RealmID: "r", ClientID: "svc", ClientSecret: "wrong",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestClientCredentials_PublicClientRejected(t *testing.T) {
	clients, _, _, _, _, uc := newOAuth(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)

	_, err := uc.ClientCredentials(context.Background(), app.ClientCredentialsInput{
		RealmID: "r", ClientID: "web",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestClientCredentials_ScopeNotAllowed(t *testing.T) {
	clients, _, _, _, _, uc := newOAuth(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(serviceClient(hashSecret("s3cret")), nil)

	_, err := uc.ClientCredentials(context.Background(), app.ClientCredentialsInput{
		RealmID: "r", ClientID: "svc", ClientSecret: "s3cret", Scopes: []string{"admin.everything"},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestClientCredentials_GrantNotAllowed(t *testing.T) {
	clients, _, _, _, _, uc := newOAuth(t)
	// A confidential client without the client_credentials grant configured.
	clients.EXPECT().Get(mock.Anything, mock.Anything).
		Return(confidentialWeb(hashSecret("s3cret")), nil)

	_, err := uc.ClientCredentials(context.Background(), app.ClientCredentialsInput{
		RealmID: "r", ClientID: "web", ClientSecret: "s3cret",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRefresh_Success(t *testing.T) {
	clients, _, _, refresh, sessions, keys, uc := newOAuthRefresh(t)
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("the-refresh")).Return(domain.RefreshToken{
		ID: "rt-1", SessionID: "sess-1", ClientID: "web", Scopes: []string{"trade.place"},
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil)
	refresh.EXPECT().MarkUsed(mock.Anything, "rt-1", mock.Anything).Return(true, nil)
	sessions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewSession(internaltest.WithSessionID("sess-1"), internaltest.WithSessionRealmID("r"), internaltest.WithSessionAccountID("acc-1")), nil)
	refresh.EXPECT().Create(mock.Anything, mock.MatchedBy(func(rt domain.RefreshToken) bool {
		return rt.RotatedFrom == "rt-1" && rt.SessionID == "sess-1" && rt.ClientID == "web" &&
			rt.TokenHash != "" && len(rt.Scopes) == 1 && rt.Scopes[0] == "trade.place"
	})).Return(nil)

	resp, err := uc.Refresh(context.Background(), app.RefreshInput{
		RealmID: "r", Issuer: "https://auth/realms/r", ClientID: "web", RefreshToken: "the-refresh",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken, "rotation yields a new refresh token")
	assert.Empty(t, resp.IDToken, "no openid scope yields no id_token")
}

func TestRefresh_ReuseRevokesSession(t *testing.T) {
	clients, _, _, refresh, sessions, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	used := time.Now().Add(-time.Minute)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("reused")).Return(domain.RefreshToken{
		ID: "rt-1", SessionID: "sess-1", ClientID: "web", UsedAt: &used,
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil)
	sessions.EXPECT().Revoke(mock.Anything, "sess-1", mock.Anything).Return(nil)

	_, err := uc.Refresh(context.Background(), app.RefreshInput{
		RealmID: "r", ClientID: "web", RefreshToken: "reused",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRefresh_Expired(t *testing.T) {
	clients, _, _, refresh, _, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("stale")).Return(domain.RefreshToken{
		ID: "rt-1", SessionID: "sess-1", ClientID: "web", ExpiresAt: time.Now().Add(-time.Minute),
	}, nil)

	_, err := uc.Refresh(context.Background(), app.RefreshInput{
		RealmID: "r", ClientID: "web", RefreshToken: "stale",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRefresh_UnknownToken(t *testing.T) {
	clients, _, _, refresh, _, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("nope")).
		Return(domain.RefreshToken{}, apierrors.NotFound("refresh token", ""))

	_, err := uc.Refresh(context.Background(), app.RefreshInput{
		RealmID: "r", ClientID: "web", RefreshToken: "nope",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRefresh_WrongClient(t *testing.T) {
	clients, _, _, refresh, _, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("mine")).Return(domain.RefreshToken{
		ID: "rt-1", SessionID: "sess-1", ClientID: "other", ExpiresAt: time.Now().Add(time.Hour),
	}, nil)

	_, err := uc.Refresh(context.Background(), app.RefreshInput{
		RealmID: "r", ClientID: "web", RefreshToken: "mine",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestIntrospect_ActiveAccessToken(t *testing.T) {
	clients, _, _, _, _, keys, uc := newOAuthRefresh(t)
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	tok, _, err := app.NewTokenIssuer(keys).MintAccessToken(context.Background(), "r", app.AccessTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", Audience: "web", ClientID: "web",
		Scopes: []string{"openid"}, TTL: time.Hour,
	})
	require.NoError(t, err)
	keys.EXPECT().VerificationKey(mock.Anything, "r", "kid-1").Return(&key.PublicKey, nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)

	res, err := uc.Introspect(context.Background(), app.IntrospectInput{RealmID: "r", ClientID: "web", Token: tok})
	require.NoError(t, err)
	assert.True(t, res.Active)
	assert.Equal(t, "acc-1", res.Subject)
	assert.Equal(t, "openid", res.Scope)
	assert.Equal(t, "Bearer", res.TokenType)
}

func TestIntrospect_ActiveRefreshToken(t *testing.T) {
	clients, _, _, refresh, sessions, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("opaque-refresh")).Return(domain.RefreshToken{
		ID: "rt-1", SessionID: "sess-1", ClientID: "web", Scopes: []string{"openid"},
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil)
	sessions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewSession(internaltest.WithSessionID("sess-1"), internaltest.WithSessionRealmID("r"), internaltest.WithSessionAccountID("acc-1")), nil)

	res, err := uc.Introspect(context.Background(), app.IntrospectInput{RealmID: "r", ClientID: "web", Token: "opaque-refresh"})
	require.NoError(t, err)
	assert.True(t, res.Active)
	assert.Equal(t, "refresh_token", res.TokenType)
	assert.Equal(t, "acc-1", res.Subject)
}

func TestIntrospect_InactiveUnknownToken(t *testing.T) {
	clients, _, _, refresh, _, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("garbage")).
		Return(domain.RefreshToken{}, apierrors.NotFound("refresh token", ""))

	res, err := uc.Introspect(context.Background(), app.IntrospectInput{RealmID: "r", ClientID: "web", Token: "garbage"})
	require.NoError(t, err)
	assert.False(t, res.Active)
}

func TestIntrospect_BadClient(t *testing.T) {
	clients, _, _, _, _, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(confidentialWeb(hashSecret("right")), nil)

	_, err := uc.Introspect(context.Background(), app.IntrospectInput{
		RealmID: "r", ClientID: "web", ClientSecret: "wrong", Token: "x",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestRevoke_RefreshTokenRevokesSession(t *testing.T) {
	clients, _, _, refresh, sessions, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("opaque")).
		Return(domain.RefreshToken{ID: "rt-1", SessionID: "sess-1", ClientID: "web"}, nil)
	sessions.EXPECT().Revoke(mock.Anything, "sess-1", mock.Anything).Return(nil)

	require.NoError(t, uc.Revoke(context.Background(), app.RevokeInput{RealmID: "r", ClientID: "web", Token: "opaque"}))
}

func TestRevoke_UnknownTokenSucceeds(t *testing.T) {
	clients, _, _, refresh, _, _, uc := newOAuthRefresh(t)
	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(publicWeb(), nil)
	refresh.EXPECT().GetByHash(mock.Anything, hashSecret("nope")).
		Return(domain.RefreshToken{}, apierrors.NotFound("refresh token", ""))

	// RFC 7009: revoking an unknown token still succeeds.
	require.NoError(t, uc.Revoke(context.Background(), app.RevokeInput{RealmID: "r", ClientID: "web", Token: "nope"}))
}

func TestSession_StartAndResolve(t *testing.T) {
	_, _, _, sessions, _, uc := newOAuth(t)
	created := internaltest.NewSession(internaltest.WithSessionID("sess-1"), internaltest.WithSessionRealmID("r"), internaltest.WithSessionAccountID("acc-1"))
	sessions.EXPECT().Create(mock.Anything, mock.Anything).Return(created, nil)
	sessions.EXPECT().Get(mock.Anything, mock.Anything).Return(created, nil)

	sid, err := uc.StartSession(context.Background(), "r", "acc-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", sid)

	got, err := uc.ResolveSession(context.Background(), "sess-1")
	require.NoError(t, err)
	assert.Equal(t, "acc-1", got.AccountID())
}

func TestSession_ResolveExpired(t *testing.T) {
	_, _, _, sessions, _, uc := newOAuth(t)
	sessions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewSession(internaltest.WithSessionRealmID("r"), internaltest.WithSessionAccountID("acc-1"), internaltest.WithSessionExpiresAt(time.Now().Add(-time.Minute))), nil)

	_, err := uc.ResolveSession(context.Background(), "sess-1")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}
