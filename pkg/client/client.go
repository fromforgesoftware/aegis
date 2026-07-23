// Package client is the consumer-facing SDK for Aegis. Every Aegis API a
// service can consume is exposed as a small typed interface (one per API),
// aggregated under Client:
//
//   - gRPC (low-latency S2S, unauthenticated — subject ids are explicit
//     request fields): AdminAPI, AuthxAPI, OAuthAPI, IdentityAPI,
//     AuthorizerAPI, MFAAPI.
//   - HTTP JSON:API (admin/write surface, gated by the Foundry gateway
//     HMAC bearer when FORGE_GATEWAY_SECRET is set): ResourceAPI,
//     BindingAPI, RoleAPI, PermissionAPI, GroupAPI, AuthorizationAdminAPI.
//
// Wire everything with Dial (plain Go) or FxModule (fx apps); each API can
// also be constructed individually against an existing connection. The
// middleware subpackage wraps IdentityAPI into drop-in gRPC/HTTP request
// middleware.
package client

// Client aggregates the per-API clients. Accessors return nil when the
// transport backing that API was not configured (e.g. an HTTP-only client
// has no AuthorizerAPI).
type Client interface {
	// gRPC surfaces.
	AdminAPI() AdminAPI
	AuthxAPI() AuthxAPI
	OAuthAPI() OAuthAPI
	IdentityAPI() IdentityAPI
	AuthorizerAPI() AuthorizerAPI
	MFAAPI() MFAAPI

	// HTTP JSON:API surfaces.
	ResourceAPI() ResourceAPI
	BindingAPI() BindingAPI
	RoleAPI() RoleAPI
	PermissionAPI() PermissionAPI
	GroupAPI() GroupAPI
	OrganizationAPI() OrganizationAPI
	AuthorizationAdminAPI() AuthorizationAdminAPI
}

type client struct {
	admin      AdminAPI
	authx      AuthxAPI
	oauth      OAuthAPI
	identity   IdentityAPI
	authorizer AuthorizerAPI
	mfa        MFAAPI

	resource     ResourceAPI
	binding      BindingAPI
	role         RoleAPI
	permission   PermissionAPI
	group        GroupAPI
	organization OrganizationAPI
	authzAdmin   AuthorizationAdminAPI
}

func NewClient(
	admin AdminAPI,
	authx AuthxAPI,
	oauth OAuthAPI,
	identity IdentityAPI,
	authorizer AuthorizerAPI,
	mfa MFAAPI,
	resource ResourceAPI,
	binding BindingAPI,
	role RoleAPI,
	permission PermissionAPI,
	group GroupAPI,
	organization OrganizationAPI,
	authzAdmin AuthorizationAdminAPI,
) *client {
	return &client{
		admin:        admin,
		authx:        authx,
		oauth:        oauth,
		identity:     identity,
		authorizer:   authorizer,
		mfa:          mfa,
		resource:     resource,
		binding:      binding,
		role:         role,
		permission:   permission,
		group:        group,
		organization: organization,
		authzAdmin:   authzAdmin,
	}
}

func (c *client) AdminAPI() AdminAPI                           { return c.admin }
func (c *client) AuthxAPI() AuthxAPI                           { return c.authx }
func (c *client) OAuthAPI() OAuthAPI                           { return c.oauth }
func (c *client) IdentityAPI() IdentityAPI                     { return c.identity }
func (c *client) AuthorizerAPI() AuthorizerAPI                 { return c.authorizer }
func (c *client) MFAAPI() MFAAPI                               { return c.mfa }
func (c *client) ResourceAPI() ResourceAPI                     { return c.resource }
func (c *client) BindingAPI() BindingAPI                       { return c.binding }
func (c *client) RoleAPI() RoleAPI                             { return c.role }
func (c *client) PermissionAPI() PermissionAPI                 { return c.permission }
func (c *client) GroupAPI() GroupAPI                           { return c.group }
func (c *client) OrganizationAPI() OrganizationAPI             { return c.organization }
func (c *client) AuthorizationAdminAPI() AuthorizationAdminAPI { return c.authzAdmin }

// ResolvedAccount is the identity an upstream token maps to.
type ResolvedAccount struct {
	AccountID    string
	Email        string
	DisplayName  string
	Created      bool
	LinkRequired bool
}
