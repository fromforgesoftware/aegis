package app

import (
	"context"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// appleDiscoveryURL is the well-known OIDC discovery doc for Sign in with Apple.
const appleDiscoveryURL = "https://appleid.apple.com/.well-known/openid-configuration"

// AppleConnector verifies Sign-in-with-Apple ID tokens. Apple is OIDC-shaped,
// so verification delegates to OIDCConnector with Apple's discovery URL baked
// in. Authorization-code exchange (which needs the JWT-signed client_secret
// derived from Apple's developer key) lands later once we wire the full
// redirect flow; today the client app does the redeem and hands us the ID
// token.
type AppleConnector struct {
	oidc *OIDCConnector
}

func NewAppleConnector(http HTTPDoer) *AppleConnector {
	return &AppleConnector{oidc: NewOIDCConnector(http)}
}

func (c *AppleConnector) Kind() domain.ExternalIDPKind { return domain.ExternalIDPKindOAuthApple }

func (c *AppleConnector) Verify(ctx context.Context, cfg domain.ExternalIDPConfig, rawToken string) (ExternalUser, error) {
	return c.oidc.Verify(ctx, withDefaultDiscoveryURL(cfg, appleDiscoveryURL), rawToken)
}
