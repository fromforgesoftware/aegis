package domain

import "github.com/fromforgesoftware/go-kit/resource"

// ResourceTypeQuotaPolicy is the JSON:API type for /api/quota-policies.
const ResourceTypeQuotaPolicy resource.Type = "quotaPolicies"

// QuotaPolicy caps how many of a countable resource a realm may hold
// (e.g. characters per game-realm).
type QuotaPolicy interface {
	resource.Resource
	RealmID() string
	ResourceType() string
	MaxCount() int
}

type quotaPolicy struct {
	resource.Resource

	realmID      string
	resourceType string
	maxCount     int
}

type QuotaPolicyOption func(*quotaPolicy)

func WithQuotaPolicyID(id string) QuotaPolicyOption {
	return func(p *quotaPolicy) { p.Resource = resource.Update(p.Resource, resource.WithID(id)) }
}

func NewQuotaPolicy(realmID, resourceType string, maxCount int, opts ...QuotaPolicyOption) QuotaPolicy {
	p := &quotaPolicy{
		Resource:     resource.New(resource.WithType(ResourceTypeQuotaPolicy)),
		realmID:      realmID,
		resourceType: resourceType,
		maxCount:     maxCount,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *quotaPolicy) RealmID() string      { return p.realmID }
func (p *quotaPolicy) ResourceType() string { return p.resourceType }
func (p *quotaPolicy) MaxCount() int        { return p.maxCount }
