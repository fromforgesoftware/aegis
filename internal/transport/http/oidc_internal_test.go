package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// These cover the pure discovery/issuer logic directly (no handler under
// test, so no HTTP harness needed).

func TestIssuerURL_HonoursForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/realms/r/.well-known/openid-configuration", http.NoBody)
	req.SetPathValue("realm", "trading-bot")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "auth.example.com")

	assert.Equal(t, "https://auth.example.com/realms/trading-bot", issuerURL(req))
}

func TestDiscoveryDoc(t *testing.T) {
	issuer := "https://auth.example.com/realms/trading-bot"
	doc := discoveryDoc(issuer)

	assert.Equal(t, issuer, doc.Issuer)
	assert.Equal(t, issuer+"/.well-known/jwks.json", doc.JWKSURI)
	assert.Equal(t, issuer+"/authorize", doc.AuthorizationEndpoint)
	assert.Equal(t, issuer+"/token", doc.TokenEndpoint)
	assert.Contains(t, doc.IDTokenSigningAlgValuesSupported, "RS256")
	assert.Contains(t, doc.CodeChallengeMethodsSupported, "S256")
}
