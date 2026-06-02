package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeRealm is the JSON:API type for /api/realms.
const ResourceTypeRealm resource.Type = "realms"

// Realm is the top-level isolation boundary; accounts, roles, clients, and
// resources all belong to one.
type Realm interface {
	resource.Resource
	Name() string
	DisplayName() string
}

type realm struct {
	resource.Resource

	name        string
	displayName string
}

type RealmOption func(*realm)

func WithRealmID(id string) RealmOption {
	return func(r *realm) { r.Resource = resource.Update(r.Resource, resource.WithID(id)) }
}
func WithRealmDisplayName(d string) RealmOption {
	return func(r *realm) { r.displayName = d }
}

// NewRealm builds a realm aggregate; name is mandatory and unique.
func NewRealm(name string, opts ...RealmOption) Realm {
	r := &realm{
		Resource: resource.New(resource.WithType(ResourceTypeRealm)),
		name:     name,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *realm) Name() string        { return r.name }
func (r *realm) DisplayName() string { return r.displayName }
