package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
)

func TestAccessToken_CarriesOrgClaims(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)

	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	keys.EXPECT().VerificationKey(mock.Anything, "r", "kid-1").Return(&key.PublicKey, nil)

	issuer := app.NewTokenIssuer(keys)
	tok, _, err := issuer.MintAccessToken(context.Background(), "r", app.AccessTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", Audience: "web", ClientID: "web",
		Scopes: []string{"openid"}, OrgID: "org-1", OrgRole: "Owner", TTL: time.Minute,
	})
	require.NoError(t, err)

	claims, err := issuer.VerifyAccessToken(context.Background(), "r", tok)
	require.NoError(t, err)
	assert.Equal(t, "org-1", claims.OrgID)
	assert.Equal(t, "Owner", claims.OrgRole)
}

func TestAccessToken_OmitsOrgWhenEmpty(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)

	keys := apptest.NewSigningKeyService(t)
	keys.EXPECT().ActiveSigner(mock.Anything, "r").Return(app.Signer{Kid: "kid-1", Key: key}, nil)
	keys.EXPECT().VerificationKey(mock.Anything, "r", "kid-1").Return(&key.PublicKey, nil)

	issuer := app.NewTokenIssuer(keys)
	tok, _, err := issuer.MintAccessToken(context.Background(), "r", app.AccessTokenInput{
		Issuer: "https://auth/realms/r", Subject: "acc-1", Audience: "web", ClientID: "web", TTL: time.Minute,
	})
	require.NoError(t, err)

	claims, err := issuer.VerifyAccessToken(context.Background(), "r", tok)
	require.NoError(t, err)
	assert.Empty(t, claims.OrgID)
	assert.Empty(t, claims.OrgRole)
}
