package app

import (
	"context"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// googleDiscoveryURL is the well-known OIDC discovery doc for accounts.google.com.
const googleDiscoveryURL = "https://accounts.google.com/.well-known/openid-configuration"

// GoogleConnector verifies Google ID tokens. Google is a standard OIDC
// provider, so verification delegates to OIDCConnector with the well-known
// discovery URL baked in — admins don't need to configure it.
type GoogleConnector struct {
	oidc *OIDCConnector
}

func NewGoogleConnector(http HTTPDoer) *GoogleConnector {
	return &GoogleConnector{oidc: NewOIDCConnector(http)}
}

func (c *GoogleConnector) Kind() domain.ExternalIDPKind { return domain.ExternalIDPKindOAuthGoogle }

func (c *GoogleConnector) Verify(ctx context.Context, cfg domain.ExternalIDPConfig, rawToken string) (ExternalUser, error) {
	return c.oidc.Verify(ctx, withDefaultDiscoveryURL(cfg, googleDiscoveryURL), rawToken)
}

// withDefaultDiscoveryURL returns a clone of cfg with DiscoveryURL set to fallback when empty.
func withDefaultDiscoveryURL(cfg domain.ExternalIDPConfig, fallback string) domain.ExternalIDPConfig {
	if cfg.DiscoveryURL() != "" {
		return cfg
	}
	return domain.NewExternalIDPConfig(cfg.RealmID(), cfg.Kind(), cfg.Name(),
		domain.WithExternalIDPID(cfg.ID()),
		domain.WithExternalIDPEnabled(cfg.Enabled()),
		domain.WithExternalIDPClientID(cfg.ClientID()),
		domain.WithExternalIDPClientSecretEncrypted(cfg.ClientSecretEncrypted()),
		domain.WithExternalIDPDiscoveryURL(fallback),
		domain.WithExternalIDPIssuer(cfg.Issuer()),
		domain.WithExternalIDPScopes(cfg.Scopes()),
		domain.WithExternalIDPConfig(cfg.Config()),
	)
}
