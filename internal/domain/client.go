package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeClient is the JSON:API type for /api/clients.
const ResourceTypeClient resource.Type = "clients"

// ClientType discriminates public clients (no secret; PKCE) from
// confidential clients (hold a secret).
type ClientType string

const (
	ClientTypePublic       ClientType = "PUBLIC"
	ClientTypeConfidential ClientType = "CONFIDENTIAL"
)

func (t ClientType) Valid() bool {
	switch t {
	case ClientTypePublic, ClientTypeConfidential:
		return true
	}
	return false
}

// Client is an OIDC/OAuth2 client application registered in a realm. The
// resource ID is the UUID; ClientID is the OAuth client_id attribute.
// SecretHash is the stored hash (confidential clients); Secret is the raw
// value surfaced exactly once on creation and never persisted/serialized
// afterwards.
type Client interface {
	resource.Resource
	RealmID() string
	ClientID() string
	ClientType() ClientType
	Name() string
	GrantTypes() []string
	Scopes() []string
	RedirectURIs() []string
	PKCERequired() bool
	SecretHash() string
	Secret() string
}

type client struct {
	resource.Resource

	realmID      string
	clientID     string
	clientType   ClientType
	name         string
	grantTypes   []string
	scopes       []string
	redirectURIs []string
	pkceRequired bool
	secretHash   string
	secret       string // transient: only right after create
}

type ClientOption func(*client)

func WithClientID(id string) ClientOption {
	return func(c *client) { c.Resource = resource.Update(c.Resource, resource.WithID(id)) }
}
func WithClientGrantTypes(g []string) ClientOption {
	return func(c *client) { c.grantTypes = g }
}
func WithClientScopes(s []string) ClientOption {
	return func(c *client) { c.scopes = s }
}
func WithClientRedirectURIs(u []string) ClientOption {
	return func(c *client) { c.redirectURIs = u }
}
func WithClientPKCERequired(v bool) ClientOption {
	return func(c *client) { c.pkceRequired = v }
}
func WithClientSecretHash(h string) ClientOption {
	return func(c *client) { c.secretHash = h }
}
func WithClientSecret(s string) ClientOption {
	return func(c *client) { c.secret = s }
}

// NewClient builds a client aggregate. clientID + type + name are
// mandatory; PKCE defaults on (public clients should always use it).
func NewClient(realmID, clientID string, clientType ClientType, name string, opts ...ClientOption) Client {
	c := &client{
		Resource:     resource.New(resource.WithType(ResourceTypeClient)),
		realmID:      realmID,
		clientID:     clientID,
		clientType:   clientType,
		name:         name,
		pkceRequired: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *client) RealmID() string        { return c.realmID }
func (c *client) ClientID() string       { return c.clientID }
func (c *client) ClientType() ClientType { return c.clientType }
func (c *client) Name() string           { return c.name }
func (c *client) GrantTypes() []string   { return c.grantTypes }
func (c *client) Scopes() []string       { return c.scopes }
func (c *client) RedirectURIs() []string { return c.redirectURIs }
func (c *client) PKCERequired() bool     { return c.pkceRequired }
func (c *client) SecretHash() string     { return c.secretHash }
func (c *client) Secret() string         { return c.secret }
