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

// The Google connector reuses OIDCConnector with Google's discovery URL
// baked in — verify that the admin's config doesn't need DiscoveryURL set
// and that the verification flow still terminates against the upstream's
// JWKS.
func TestGoogleConnector_UsesBakedDiscoveryURL(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintUpstreamIDToken(t, key, "g-kid", "https://accounts.google.com", "aegis-client", "google-uid-9", "g@b.com")
	http := &fakeHTTP{responses: map[string]string{
		"https://accounts.google.com/.well-known/openid-configuration": mustJSON(t, map[string]string{
			"issuer": "https://accounts.google.com", "jwks_uri": "https://www.googleapis.com/oauth2/v3/certs",
		}),
		"https://www.googleapis.com/oauth2/v3/certs": upstreamJWKS(t, &key.PublicKey, "g-kid"),
	}}

	cfg := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
		internaltest.WithExternalIDPName("google-prod"),
		internaltest.WithExternalIDPClientID("aegis-client"),
	)
	user, err := app.NewGoogleConnector(http).Verify(t.Context(), cfg, idToken)
	require.NoError(t, err)
	assert.Equal(t, "google-uid-9", user.ID)
	assert.Equal(t, domain.ExternalIDPKindOAuthGoogle, app.NewGoogleConnector(http).Kind())
}

func TestGoogleConnector_AdminOverrideDiscoveryURLWins(t *testing.T) {
	// An admin-provided DiscoveryURL takes precedence — useful for staging
	// against a Google emulator or proxy.
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	idToken := mintUpstreamIDToken(t, key, "g-kid", "https://staging.example", "aegis-client", "u", "g@b.com")
	http := &fakeHTTP{responses: map[string]string{
		"https://staging.example/.well-known/openid-configuration": mustJSON(t, map[string]string{
			"issuer": "https://staging.example", "jwks_uri": "https://staging.example/jwks.json",
		}),
		"https://staging.example/jwks.json": upstreamJWKS(t, &key.PublicKey, "g-kid"),
	}}

	cfg := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
		internaltest.WithExternalIDPName("google-staging"),
		internaltest.WithExternalIDPClientID("aegis-client"),
		internaltest.WithExternalIDPDiscoveryURL("https://staging.example/.well-known/openid-configuration"),
	)
	_, err = app.NewGoogleConnector(http).Verify(t.Context(), cfg, idToken)
	require.NoError(t, err)
}
