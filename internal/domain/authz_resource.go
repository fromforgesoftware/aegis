package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeAuthzResource is the JSON:API type for /api/resources — the
// authz resource registry (named "AuthzResource" in code to keep the kit's
// resource.Resource interface unambiguous).
const ResourceTypeAuthzResource resource.Type = "resources"

// Visibility classifies whether a resource is accessible only via explicit
// bindings (PRIVATE) or open to every realm member (PUBLIC). Cross-resource
// SHARED links land in a later slice.
type Visibility string

const (
	VisibilityPrivate Visibility = "PRIVATE"
	VisibilityPublic  Visibility = "PUBLIC"
)

func (v Visibility) Valid() bool {
	switch v {
	case VisibilityPrivate, VisibilityPublic:
		return true
	}
	return false
}

// AuthzResource is a registered domain object inside Aegis's resource
// hierarchy. parent_id walks up the hierarchy for inheritance; inherit_via
// labels the edge for the MV-closure walk.
type AuthzResource interface {
	resource.Resource
	RealmID() string
	ResourceType() string
	OwnerAccountID() string
	ParentID() string
	InheritVia() string
	Visibility() Visibility
}

type authzResource struct {
	resource.Resource

	realmID        string
	resourceType   string
	ownerAccountID string
	parentID       string
	inheritVia     string
	visibility     Visibility
}

type AuthzResourceOption func(*authzResource)

func WithAuthzResourceID(id string) AuthzResourceOption {
	return func(r *authzResource) { r.Resource = resource.Update(r.Resource, resource.WithID(id)) }
}
func WithAuthzResourceOwnerAccountID(id string) AuthzResourceOption {
	return func(r *authzResource) { r.ownerAccountID = id }
}
func WithAuthzResourceParentID(id string) AuthzResourceOption {
	return func(r *authzResource) { r.parentID = id }
}
func WithAuthzResourceInheritVia(v string) AuthzResourceOption {
	return func(r *authzResource) { r.inheritVia = v }
}
func WithAuthzResourceVisibility(v Visibility) AuthzResourceOption {
	return func(r *authzResource) { r.visibility = v }
}

// NewAuthzResource builds a resource aggregate; realmID + resourceType are
// mandatory, visibility defaults to PRIVATE.
func NewAuthzResource(realmID, resourceType string, opts ...AuthzResourceOption) AuthzResource {
	r := &authzResource{
		Resource:     resource.New(resource.WithType(ResourceTypeAuthzResource)),
		realmID:      realmID,
		resourceType: resourceType,
		visibility:   VisibilityPrivate,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *authzResource) RealmID() string        { return r.realmID }
func (r *authzResource) ResourceType() string   { return r.resourceType }
func (r *authzResource) OwnerAccountID() string { return r.ownerAccountID }
func (r *authzResource) ParentID() string       { return r.parentID }
func (r *authzResource) InheritVia() string     { return r.inheritVia }
func (r *authzResource) Visibility() Visibility { return r.visibility }
