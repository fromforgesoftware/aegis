package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeRole is the JSON:API type for /api/roles.
const ResourceTypeRole resource.Type = "roles"

// RoleKind discriminates SYSTEM roles (shipped by Aegis or the consuming
// service) from CUSTOM roles (admin-created at runtime, GCP-IAM style).
type RoleKind string

const (
	RoleKindSystem RoleKind = "SYSTEM"
	RoleKindCustom RoleKind = "CUSTOM"
)

func (k RoleKind) Valid() bool {
	switch k {
	case RoleKindSystem, RoleKindCustom:
		return true
	}
	return false
}

// Role is a named bundle of permissions for a resource type, scoped to a
// realm. Permissions are attached via the role_permission junction.
type Role interface {
	resource.Resource
	RealmID() string
	Name() string
	ResourceType() string
	Description() string
	Kind() RoleKind
}

type role struct {
	resource.Resource

	realmID      string
	name         string
	resourceType string
	description  string
	kind         RoleKind
}

type RoleOption func(*role)

func WithRoleID(id string) RoleOption {
	return func(r *role) { r.Resource = resource.Update(r.Resource, resource.WithID(id)) }
}
func WithRoleDescription(d string) RoleOption {
	return func(r *role) { r.description = d }
}
func WithRoleKind(k RoleKind) RoleOption {
	return func(r *role) { r.kind = k }
}

// NewRole builds a role aggregate. realmID/name/resourceType are mandatory;
// kind defaults to CUSTOM.
func NewRole(realmID, name, resourceType string, opts ...RoleOption) Role {
	r := &role{
		Resource:     resource.New(resource.WithType(ResourceTypeRole)),
		realmID:      realmID,
		name:         name,
		resourceType: resourceType,
		kind:         RoleKindCustom,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *role) RealmID() string      { return r.realmID }
func (r *role) Name() string         { return r.name }
func (r *role) ResourceType() string { return r.resourceType }
func (r *role) Description() string  { return r.description }
func (r *role) Kind() RoleKind       { return r.kind }
