package app_test

import (
	"context"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
)

func parseRS256(t *testing.T, token string, pub *rsa.PublicKey) (*jwt.Token, jwt.MapClaims) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return pub, nil },
		jwt.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	return parsed, claims
}

func TestMintAccessToken(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)

	tok, expiresIn, err := app.NewTokenIssuer(keys).MintAccessToken(context.Background(), "r", app.AccessTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", Audience: "web", ClientID: "web",
		Scopes: []string{"openid", "profile"}, TTL: time.Hour,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3600), expiresIn)

	parsed, claims := parseRS256(t, tok, &key.PublicKey)
	assert.Equal(t, "kid-1", parsed.Header["kid"])
	assert.Equal(t, "at+jwt", parsed.Header["typ"])
	assert.Equal(t, "https://auth/realms/r", claims["iss"])
	assert.Equal(t, "acc-1", claims["sub"])
	assert.Equal(t, "openid profile", claims["scope"])
	assert.NotEmpty(t, claims["jti"])
}

func TestVerifyAccessToken_RoundTrip(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	keys.EXPECT().VerificationKey(mock.Anything, "r", "kid-1").Return(&key.PublicKey, nil)

	issuer := app.NewTokenIssuer(keys)
	tok, _, err := issuer.MintAccessToken(context.Background(), "r", app.AccessTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", Audience: "web", ClientID: "web",
		Scopes: []string{"openid"}, TTL: time.Hour,
	})
	require.NoError(t, err)

	claims, err := issuer.VerifyAccessToken(context.Background(), "r", tok)
	require.NoError(t, err)
	assert.Equal(t, "acc-1", claims.Subject)
	assert.Equal(t, "web", claims.ClientID)
	assert.Equal(t, "openid", claims.Scope)
	assert.Equal(t, "https://auth/realms/r", claims.Issuer)
}

func TestVerifyAccessToken_WrongKeyRejected(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	other, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	keys.EXPECT().VerificationKey(mock.Anything, "r", "kid-1").Return(&other.PublicKey, nil)

	issuer := app.NewTokenIssuer(keys)
	tok, _, err := issuer.MintAccessToken(context.Background(), "r", app.AccessTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", TTL: time.Hour,
	})
	require.NoError(t, err)

	_, err = issuer.VerifyAccessToken(context.Background(), "r", tok)
	require.Error(t, err)
}

func TestMintIDToken(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)

	tok, err := app.NewTokenIssuer(keys).MintIDToken(context.Background(), "r", app.IDTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", Audience: "web", Nonce: "n-123",
		Email: "a@b.com", EmailVerified: true, Name: "Alice", TTL: time.Hour,
	})
	require.NoError(t, err)

	_, claims := parseRS256(t, tok, &key.PublicKey)
	assert.Equal(t, "web", claims["aud"])
	assert.Equal(t, "n-123", claims["nonce"])
	assert.Equal(t, "a@b.com", claims["email"])
	assert.Equal(t, true, claims["email_verified"])
	assert.Equal(t, "Alice", claims["name"])
}
