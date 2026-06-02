package app_test

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// fakeHTTP serves an in-memory map of URL → response body to the connector
// suites so tests run with no real network. It also records the most recent
// request's headers, which the GitHub-style connectors assert on.
type fakeHTTP struct {
	responses map[string]string
	errs      map[string]error
	lastReq   *http.Request
}

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req
	url := req.URL.String()
	if err, ok := f.errs[url]; ok {
		return nil, err
	}
	body, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func upstreamJWKS(t *testing.T, pub *rsa.PublicKey, kid string) string {
	t.Helper()
	jwk := jose.JSONWebKey{Key: pub, KeyID: kid, Algorithm: "RS256", Use: "sig"}
	raw, err := jwk.MarshalJSON()
	require.NoError(t, err)
	return fmt.Sprintf(`{"keys":[%s]}`, string(raw))
}

func mintUpstreamIDToken(t *testing.T, key *rsa.PrivateKey, kid, iss, aud, sub, email string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":            iss,
		"aud":            aud,
		"sub":            sub,
		"email":          email,
		"email_verified": true,
		"name":           "Upstream User",
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	require.NoError(t, err)
	return s
}

func customOIDCConfig() domain.ExternalIDPConfig {
	return internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOIDCCustom),
		internaltest.WithExternalIDPName("upstream"),
		internaltest.WithExternalIDPClientID("aegis-client"),
		internaltest.WithExternalIDPDiscoveryURL("https://idp.example/.well-known/openid-configuration"),
	)
}

func TestOIDCConnector_Verify_Success(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	discovery := mustJSON(t, map[string]string{
		"issuer":   "https://idp.example",
		"jwks_uri": "https://idp.example/jwks.json",
	})
	jwks := upstreamJWKS(t, &key.PublicKey, "upstream-kid")
	idToken := mintUpstreamIDToken(t, key, "upstream-kid", "https://idp.example", "aegis-client", "upstream-uid-1", "a@b.com")

	http := &fakeHTTP{responses: map[string]string{
		"https://idp.example/.well-known/openid-configuration": discovery,
		"https://idp.example/jwks.json":                        jwks,
	}}
	user, err := app.NewOIDCConnector(http).Verify(t.Context(), customOIDCConfig(), idToken)
	require.NoError(t, err)
	assert.Equal(t, "upstream-uid-1", user.ID)
	assert.Equal(t, "a@b.com", user.Email)
	assert.True(t, user.EmailVerified)
	assert.Equal(t, "Upstream User", user.Name)
}

func TestOIDCConnector_Verify_IssuerMismatch(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintUpstreamIDToken(t, key, "upstream-kid", "https://attacker.example", "aegis-client", "u", "a@b.com")
	http := &fakeHTTP{responses: map[string]string{
		"https://idp.example/.well-known/openid-configuration": mustJSON(t, map[string]string{
			"issuer": "https://idp.example", "jwks_uri": "https://idp.example/jwks.json",
		}),
		"https://idp.example/jwks.json": upstreamJWKS(t, &key.PublicKey, "upstream-kid"),
	}}
	_, err = app.NewOIDCConnector(http).Verify(t.Context(), customOIDCConfig(), idToken)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestOIDCConnector_Verify_AudienceMismatch(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintUpstreamIDToken(t, key, "upstream-kid", "https://idp.example", "different-client", "u", "a@b.com")
	http := &fakeHTTP{responses: map[string]string{
		"https://idp.example/.well-known/openid-configuration": mustJSON(t, map[string]string{
			"issuer": "https://idp.example", "jwks_uri": "https://idp.example/jwks.json",
		}),
		"https://idp.example/jwks.json": upstreamJWKS(t, &key.PublicKey, "upstream-kid"),
	}}
	_, err = app.NewOIDCConnector(http).Verify(t.Context(), customOIDCConfig(), idToken)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestOIDCConnector_Verify_UnknownKid(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintUpstreamIDToken(t, key, "rogue-kid", "https://idp.example", "aegis-client", "u", "a@b.com")
	http := &fakeHTTP{responses: map[string]string{
		"https://idp.example/.well-known/openid-configuration": mustJSON(t, map[string]string{
			"issuer": "https://idp.example", "jwks_uri": "https://idp.example/jwks.json",
		}),
		"https://idp.example/jwks.json": upstreamJWKS(t, &key.PublicKey, "upstream-kid"),
	}}
	_, err = app.NewOIDCConnector(http).Verify(t.Context(), customOIDCConfig(), idToken)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestOIDCConnector_Verify_DiscoveryUnreachable(t *testing.T) {
	http := &fakeHTTP{errs: map[string]error{
		"https://idp.example/.well-known/openid-configuration": errors.New("dial tcp"),
	}}
	_, err := app.NewOIDCConnector(http).Verify(t.Context(), customOIDCConfig(), "x")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInternalError))
}

func TestOIDCConnector_Verify_MissingDiscoveryURL(t *testing.T) {
	cfg := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOIDCCustom),
		internaltest.WithExternalIDPName("upstream"),
	)
	_, err := app.NewOIDCConnector(&fakeHTTP{}).Verify(t.Context(), cfg, "x")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
