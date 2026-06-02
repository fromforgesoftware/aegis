package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypePermission is the JSON:API type for /api/permissions.
const ResourceTypePermission resource.Type = "permissions"

// Permission is an atomic action — slug "doc.read" decomposes into a
// resource_type ("doc") + verb ("read"). The catalog is seeded by the
// consuming service's migration; Aegis ships only the DDL and the CRUD seam.
type Permission interface {
	resource.Resource
	ResourceType() string
	Verb() string
	Description() string
}

type permission struct {
	resource.Resource

	resourceType string
	verb         string
	description  string
}

type PermissionOption func(*permission)

func WithPermissionDescription(d string) PermissionOption {
	return func(p *permission) { p.description = d }
}

// NewPermission builds the catalog entry. id is the slug; resourceType+verb
// must agree with it ("doc.read" → "doc", "read").
func NewPermission(id, resourceType, verb string, opts ...PermissionOption) Permission {
	p := &permission{
		Resource:     resource.New(resource.WithType(ResourceTypePermission), resource.WithID(id)),
		resourceType: resourceType,
		verb:         verb,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *permission) ResourceType() string { return p.resourceType }
func (p *permission) Verb() string         { return p.verb }
func (p *permission) Description() string  { return p.description }
