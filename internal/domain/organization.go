package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeOrganization is the JSON:API type for /api/organizations — the
// tenant inside a realm. An organization is anchored 1:1 to an authz resource
// of type "organization" so membership, invites, and inheritance reuse the
// ReBAC engine.
const ResourceTypeOrganization resource.Type = "organizations"

// AuthzResourceTypeOrganization is the authz_resource.resource_type the org
// anchor carries; org roles and permissions bind against it.
const AuthzResourceTypeOrganization = "organization"

// OrgStatus is an organization's lifecycle state.
type OrgStatus string

const (
	OrgStatusActive    OrgStatus = "ACTIVE"
	OrgStatusSuspended OrgStatus = "SUSPENDED"
	OrgStatusArchived  OrgStatus = "ARCHIVED"
)

func (s OrgStatus) Valid() bool {
	switch s {
	case OrgStatusActive, OrgStatusSuspended, OrgStatusArchived:
		return true
	}
	return false
}

// Organization is the tenant aggregate. Relationships (realm, anchor resource,
// owner account) are exposed as resource.Identifier so the DTO renders them as
// JSON:API relationships that can be ?include=d.
type Organization interface {
	resource.Resource
	Realm() resource.Identifier
	AnchorResource() resource.Identifier
	Owner() resource.Identifier
	Name() string
	Slug() string
	Status() OrgStatus
	Settings() map[string]any
}

type organization struct {
	resource.Resource

	realmID    string
	resourceID string
	ownerID    string
	name       string
	slug       string
	status     OrgStatus
	settings   map[string]any
}

type OrganizationOption func(*organization)

func WithOrganizationID(id string) OrganizationOption {
	return func(o *organization) { o.Resource = resource.Update(o.Resource, resource.WithID(id)) }
}
func WithOrganizationResourceID(id string) OrganizationOption {
	return func(o *organization) { o.resourceID = id }
}
func WithOrganizationOwnerID(id string) OrganizationOption {
	return func(o *organization) { o.ownerID = id }
}
func WithOrganizationStatus(s OrgStatus) OrganizationOption {
	return func(o *organization) { o.status = s }
}
func WithOrganizationSettings(s map[string]any) OrganizationOption {
	return func(o *organization) { o.settings = s }
}

// NewOrganization builds an organization aggregate. realmID/name/slug are
// mandatory; status defaults to ACTIVE.
func NewOrganization(realmID, name, slug string, opts ...OrganizationOption) Organization {
	o := &organization{
		Resource: resource.New(resource.WithType(ResourceTypeOrganization)),
		realmID:  realmID,
		name:     name,
		slug:     slug,
		status:   OrgStatusActive,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *organization) Realm() resource.Identifier {
	return resource.NewIdentifier(o.realmID, ResourceTypeRealm)
}

func (o *organization) AnchorResource() resource.Identifier {
	if o.resourceID == "" {
		return nil
	}
	return resource.NewIdentifier(o.resourceID, ResourceTypeAuthzResource)
}

func (o *organization) Owner() resource.Identifier {
	if o.ownerID == "" {
		return nil
	}
	return resource.NewIdentifier(o.ownerID, ResourceTypeAccount)
}

func (o *organization) Name() string             { return o.name }
func (o *organization) Slug() string             { return o.slug }
func (o *organization) Status() OrgStatus        { return o.status }
func (o *organization) Settings() map[string]any { return o.settings }
