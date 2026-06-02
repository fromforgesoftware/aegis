package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestAppleConnector_UsesBakedDiscoveryURL(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintUpstreamIDToken(t, key, "a-kid", "https://appleid.apple.com", "aegis-client", "apple-uid-1", "a@b.com")
	http := &fakeHTTP{responses: map[string]string{
		"https://appleid.apple.com/.well-known/openid-configuration": mustJSON(t, map[string]string{
			"issuer": "https://appleid.apple.com", "jwks_uri": "https://appleid.apple.com/auth/keys",
		}),
		"https://appleid.apple.com/auth/keys": upstreamJWKS(t, &key.PublicKey, "a-kid"),
	}}

	cfg := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthApple),
		internaltest.WithExternalIDPName("apple-prod"),
		internaltest.WithExternalIDPClientID("aegis-client"),
	)
	user, err := app.NewAppleConnector(http).Verify(t.Context(), cfg, idToken)
	require.NoError(t, err)
	assert.Equal(t, "apple-uid-1", user.ID)
	assert.Equal(t, domain.ExternalIDPKindOAuthApple, app.NewAppleConnector(http).Kind())
}
