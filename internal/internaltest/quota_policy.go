package internaltest

import (
	"github.com/fromforgesoftware/aegis/internal/domain"
)

type QuotaPolicyOption func(*quotaPolicyOpts)

type quotaPolicyOpts struct {
	id           string
	realmID      string
	resourceType string
	maxCount     int
}

func defaultQuotaPolicyOptions() []QuotaPolicyOption {
	return []QuotaPolicyOption{
		WithQuotaPolicyRealmID("realm-test"),
		WithQuotaPolicyResourceType("character"),
		WithQuotaPolicyMaxCount(3),
	}
}

func WithQuotaPolicyID(id string) QuotaPolicyOption {
	return func(o *quotaPolicyOpts) { o.id = id }
}
func WithQuotaPolicyRealmID(id string) QuotaPolicyOption {
	return func(o *quotaPolicyOpts) { o.realmID = id }
}
func WithQuotaPolicyResourceType(rt string) QuotaPolicyOption {
	return func(o *quotaPolicyOpts) { o.resourceType = rt }
}
func WithQuotaPolicyMaxCount(n int) QuotaPolicyOption {
	return func(o *quotaPolicyOpts) { o.maxCount = n }
}

func NewQuotaPolicy(opts ...QuotaPolicyOption) domain.QuotaPolicy {
	o := &quotaPolicyOpts{}
	for _, opt := range append(defaultQuotaPolicyOptions(), opts...) {
		opt(o)
	}
	var domainOpts []domain.QuotaPolicyOption
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithQuotaPolicyID(o.id))
	}
	return domain.NewQuotaPolicy(o.realmID, o.resourceType, o.maxCount, domainOpts...)
}

// MatchQuotaPolicy compares realm + resource_type + max, ignoring id.
func MatchQuotaPolicy(want domain.QuotaPolicy) func(domain.QuotaPolicy) bool {
	return func(got domain.QuotaPolicy) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.ResourceType() == got.ResourceType() &&
			want.MaxCount() == got.MaxCount()
	}
}
