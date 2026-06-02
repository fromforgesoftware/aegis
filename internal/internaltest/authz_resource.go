package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type AuthzResourceOption func(*authzResourceOpts)

type authzResourceOpts struct {
	id             string
	realmID        string
	resourceType   string
	ownerAccountID string
	parentID       string
	inheritVia     string
	visibility     domain.Visibility
}

func defaultAuthzResourceOptions() []AuthzResourceOption {
	return []AuthzResourceOption{
		WithAuthzResourceRealmID("realm-test"),
		WithAuthzResourceResourceType("doc"),
		WithAuthzResourceVisibility(domain.VisibilityPrivate),
	}
}

func WithAuthzResourceID(id string) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.id = id }
}
func WithAuthzResourceRealmID(id string) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.realmID = id }
}
func WithAuthzResourceResourceType(rt string) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.resourceType = rt }
}
func WithAuthzResourceOwnerAccountID(id string) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.ownerAccountID = id }
}
func WithAuthzResourceParentID(id string) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.parentID = id }
}
func WithAuthzResourceInheritVia(v string) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.inheritVia = v }
}
func WithAuthzResourceVisibility(v domain.Visibility) AuthzResourceOption {
	return func(o *authzResourceOpts) { o.visibility = v }
}

func NewAuthzResource(opts ...AuthzResourceOption) domain.AuthzResource {
	o := &authzResourceOpts{}
	for _, opt := range append(defaultAuthzResourceOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.AuthzResourceOption{
		domain.WithAuthzResourceOwnerAccountID(o.ownerAccountID),
		domain.WithAuthzResourceParentID(o.parentID),
		domain.WithAuthzResourceInheritVia(o.inheritVia),
		domain.WithAuthzResourceVisibility(o.visibility),
	}
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithAuthzResourceID(o.id))
	}
	return domain.NewAuthzResource(o.realmID, o.resourceType, domainOpts...)
}

// MatchAuthzResource compares realm + type + parent, ignoring id/timestamps.
func MatchAuthzResource(want domain.AuthzResource) func(domain.AuthzResource) bool {
	return func(got domain.AuthzResource) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.ResourceType() == got.ResourceType() &&
			want.ParentID() == got.ParentID()
	}
}
