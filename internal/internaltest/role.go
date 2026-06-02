package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type RoleOption func(*roleOpts)

type roleOpts struct {
	id           string
	realmID      string
	name         string
	resourceType string
	description  string
	kind         domain.RoleKind
}

func defaultRoleOptions() []RoleOption {
	return []RoleOption{
		WithRoleRealmID("realm-test"),
		WithRoleName("role-test"),
		WithRoleResourceType("doc"),
		WithRoleKind(domain.RoleKindCustom),
	}
}

func WithRoleID(id string) RoleOption      { return func(o *roleOpts) { o.id = id } }
func WithRoleRealmID(id string) RoleOption { return func(o *roleOpts) { o.realmID = id } }
func WithRoleName(n string) RoleOption     { return func(o *roleOpts) { o.name = n } }
func WithRoleResourceType(rt string) RoleOption {
	return func(o *roleOpts) { o.resourceType = rt }
}
func WithRoleDescription(d string) RoleOption   { return func(o *roleOpts) { o.description = d } }
func WithRoleKind(k domain.RoleKind) RoleOption { return func(o *roleOpts) { o.kind = k } }

func NewRole(opts ...RoleOption) domain.Role {
	o := &roleOpts{}
	for _, opt := range append(defaultRoleOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.RoleOption{
		domain.WithRoleDescription(o.description),
		domain.WithRoleKind(o.kind),
	}
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithRoleID(o.id))
	}
	return domain.NewRole(o.realmID, o.name, o.resourceType, domainOpts...)
}

// MatchRole compares realm + name + resource_type + kind, ignoring id/timestamps.
func MatchRole(want domain.Role) func(domain.Role) bool {
	return func(got domain.Role) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.Name() == got.Name() &&
			want.ResourceType() == got.ResourceType() &&
			want.Kind() == got.Kind()
	}
}
