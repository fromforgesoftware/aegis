package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeGroup is the JSON:API type for /api/groups — a named set of
// accounts ("actor_set" in Zanzibar terms, "group" to consumers) that a
// binding can grant access to so members inherit it.
const ResourceTypeGroup resource.Type = "groups"

// Group is a realm-scoped set of accounts. Members are attached via the
// actor_set_member junction. An org-scoped group (a Team) carries an
// Organization relationship; realm-level groups leave it nil.
type Group interface {
	resource.Resource
	RealmID() string
	Name() string
	Description() string
	Organization() resource.Identifier
}

type group struct {
	resource.Resource

	realmID        string
	name           string
	description    string
	organizationID string
}

type GroupOption func(*group)

func WithGroupID(id string) GroupOption {
	return func(a *group) { a.Resource = resource.Update(a.Resource, resource.WithID(id)) }
}
func WithGroupDescription(d string) GroupOption {
	return func(a *group) { a.description = d }
}
func WithGroupOrganizationID(id string) GroupOption {
	return func(a *group) { a.organizationID = id }
}

// NewGroup builds a group aggregate. realmID/name are mandatory.
func NewGroup(realmID, name string, opts ...GroupOption) Group {
	a := &group{
		Resource: resource.New(resource.WithType(ResourceTypeGroup)),
		realmID:  realmID,
		name:     name,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *group) RealmID() string     { return a.realmID }
func (a *group) Name() string        { return a.name }
func (a *group) Description() string { return a.description }
func (a *group) Organization() resource.Identifier {
	if a.organizationID == "" {
		return nil
	}
	return resource.NewIdentifier(a.organizationID, ResourceTypeOrganization)
}
