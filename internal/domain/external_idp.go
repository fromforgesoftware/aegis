package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeExternalIDP is the JSON:API type for /api/external-idps.
const ResourceTypeExternalIDP resource.Type = "externalIdps"

// ExternalIDPKind discriminates the upstream identity provider; LDAP is
// reserved for Wave 14.
type ExternalIDPKind string

const (
	ExternalIDPKindFirebase    ExternalIDPKind = "FIREBASE"
	ExternalIDPKindOAuthGoogle ExternalIDPKind = "OAUTH_GOOGLE"
	ExternalIDPKindOAuthGitHub ExternalIDPKind = "OAUTH_GITHUB"
	ExternalIDPKindOAuthApple  ExternalIDPKind = "OAUTH_APPLE"
	ExternalIDPKindOIDCCustom  ExternalIDPKind = "OIDC_CUSTOM"
	ExternalIDPKindLDAP        ExternalIDPKind = "LDAP"
)

func (k ExternalIDPKind) Valid() bool {
	switch k {
	case ExternalIDPKindFirebase, ExternalIDPKindOAuthGoogle, ExternalIDPKindOAuthGitHub,
		ExternalIDPKindOAuthApple, ExternalIDPKindOIDCCustom, ExternalIDPKindLDAP:
		return true
	}
	return false
}

// ExternalIDPConfig is a per-realm configuration for one upstream IdP.
// ClientSecretHash is the stored hash; the raw secret is never persisted.
type ExternalIDPConfig interface {
	resource.Resource
	RealmID() string
	Kind() ExternalIDPKind
	Name() string
	Enabled() bool
	ClientID() string
	ClientSecretEncrypted() []byte
	DiscoveryURL() string
	Issuer() string
	Scopes() []string
	Config() map[string]string
}

type externalIDPConfig struct {
	resource.Resource

	realmID               string
	kind                  ExternalIDPKind
	name                  string
	enabled               bool
	clientID              string
	clientSecretEncrypted []byte
	discoveryURL          string
	issuer                string
	scopes                []string
	config                map[string]string
}

type ExternalIDPConfigOption func(*externalIDPConfig)

func WithExternalIDPID(id string) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.Resource = resource.Update(c.Resource, resource.WithID(id)) }
}
func WithExternalIDPEnabled(v bool) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.enabled = v }
}
func WithExternalIDPClientID(id string) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.clientID = id }
}
func WithExternalIDPClientSecretEncrypted(b []byte) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.clientSecretEncrypted = b }
}
func WithExternalIDPDiscoveryURL(u string) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.discoveryURL = u }
}
func WithExternalIDPIssuer(i string) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.issuer = i }
}
func WithExternalIDPScopes(s []string) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.scopes = s }
}
func WithExternalIDPConfig(cfg map[string]string) ExternalIDPConfigOption {
	return func(c *externalIDPConfig) { c.config = cfg }
}

// NewExternalIDPConfig builds an IdP config aggregate; realmID/kind/name are
// mandatory.
func NewExternalIDPConfig(realmID string, kind ExternalIDPKind, name string, opts ...ExternalIDPConfigOption) ExternalIDPConfig {
	c := &externalIDPConfig{
		Resource: resource.New(resource.WithType(ResourceTypeExternalIDP)),
		realmID:  realmID,
		kind:     kind,
		name:     name,
		enabled:  true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *externalIDPConfig) RealmID() string               { return c.realmID }
func (c *externalIDPConfig) Kind() ExternalIDPKind         { return c.kind }
func (c *externalIDPConfig) Name() string                  { return c.name }
func (c *externalIDPConfig) Enabled() bool                 { return c.enabled }
func (c *externalIDPConfig) ClientID() string              { return c.clientID }
func (c *externalIDPConfig) ClientSecretEncrypted() []byte { return c.clientSecretEncrypted }
func (c *externalIDPConfig) DiscoveryURL() string          { return c.discoveryURL }
func (c *externalIDPConfig) Issuer() string                { return c.issuer }
func (c *externalIDPConfig) Scopes() []string              { return c.scopes }
func (c *externalIDPConfig) Config() map[string]string     { return c.config }
