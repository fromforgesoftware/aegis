package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type OrganizationOption func(*organizationOpts)

type organizationOpts struct {
	id         string
	realmID    string
	resourceID string
	ownerID    string
	name       string
	slug       string
	status     domain.OrgStatus
	settings   map[string]any
}

func defaultOrganizationOptions() []OrganizationOption {
	return []OrganizationOption{
		WithOrganizationRealmID("realm-test"),
		WithOrganizationName("Test Org"),
		WithOrganizationSlug("test-org"),
		WithOrganizationStatus(domain.OrgStatusActive),
	}
}

func WithOrganizationID(id string) OrganizationOption {
	return func(o *organizationOpts) { o.id = id }
}
func WithOrganizationRealmID(id string) OrganizationOption {
	return func(o *organizationOpts) { o.realmID = id }
}
func WithOrganizationResourceID(id string) OrganizationOption {
	return func(o *organizationOpts) { o.resourceID = id }
}
func WithOrganizationOwnerID(id string) OrganizationOption {
	return func(o *organizationOpts) { o.ownerID = id }
}
func WithOrganizationName(n string) OrganizationOption {
	return func(o *organizationOpts) { o.name = n }
}
func WithOrganizationSlug(s string) OrganizationOption {
	return func(o *organizationOpts) { o.slug = s }
}
func WithOrganizationStatus(s domain.OrgStatus) OrganizationOption {
	return func(o *organizationOpts) { o.status = s }
}
func WithOrganizationSettings(s map[string]any) OrganizationOption {
	return func(o *organizationOpts) { o.settings = s }
}

func NewOrganization(opts ...OrganizationOption) domain.Organization {
	o := &organizationOpts{}
	for _, opt := range append(defaultOrganizationOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.OrganizationOption{
		domain.WithOrganizationStatus(o.status),
	}
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithOrganizationID(o.id))
	}
	if o.resourceID != "" {
		domainOpts = append(domainOpts, domain.WithOrganizationResourceID(o.resourceID))
	}
	if o.ownerID != "" {
		domainOpts = append(domainOpts, domain.WithOrganizationOwnerID(o.ownerID))
	}
	if o.settings != nil {
		domainOpts = append(domainOpts, domain.WithOrganizationSettings(o.settings))
	}
	return domain.NewOrganization(o.realmID, o.name, o.slug, domainOpts...)
}

// MatchOrganization compares realm + slug + name, ignoring id/timestamps.
func MatchOrganization(want domain.Organization) func(domain.Organization) bool {
	return func(got domain.Organization) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.Realm().ID() == got.Realm().ID() &&
			want.Slug() == got.Slug() &&
			want.Name() == got.Name()
	}
}
