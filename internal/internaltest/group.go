package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type GroupOption func(*groupOpts)

type groupOpts struct {
	id          string
	realmID     string
	name        string
	description string
}

func defaultGroupOptions() []GroupOption {
	return []GroupOption{
		WithGroupRealmID("realm-test"),
		WithGroupName("group-test"),
	}
}

func WithGroupID(id string) GroupOption      { return func(o *groupOpts) { o.id = id } }
func WithGroupRealmID(id string) GroupOption { return func(o *groupOpts) { o.realmID = id } }
func WithGroupName(n string) GroupOption     { return func(o *groupOpts) { o.name = n } }
func WithGroupDescription(d string) GroupOption {
	return func(o *groupOpts) { o.description = d }
}

func NewGroup(opts ...GroupOption) domain.Group {
	o := &groupOpts{}
	for _, opt := range append(defaultGroupOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.GroupOption{
		domain.WithGroupDescription(o.description),
	}
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithGroupID(o.id))
	}
	return domain.NewGroup(o.realmID, o.name, domainOpts...)
}

// MatchGroup compares realm + name, ignoring id/timestamps.
func MatchGroup(want domain.Group) func(domain.Group) bool {
	return func(got domain.Group) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() && want.Name() == got.Name()
	}
}
