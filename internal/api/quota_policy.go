package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeQuotaPolicy is the JSON:API type for /api/quota-policies.
const ResourceTypeQuotaPolicy resource.Type = "quotaPolicies"

// QuotaPolicyDTO is the wire shape for a per-realm quota cap.
type QuotaPolicyDTO struct {
	resource.RestDTO

	RRealmID      string `jsonapi:"attr,realmId,omitempty"`
	RResourceType string `jsonapi:"attr,resourceType,omitempty"`
	RMaxCount     int    `jsonapi:"attr,maxCount"`
}

func QuotaPolicyToDTO(p domain.QuotaPolicy) *QuotaPolicyDTO {
	if p == nil {
		return nil
	}
	dto := &QuotaPolicyDTO{
		RestDTO:       resource.ToRestDTO(p),
		RRealmID:      p.RealmID(),
		RResourceType: p.ResourceType(),
		RMaxCount:     p.MaxCount(),
	}
	dto.RType = ResourceTypeQuotaPolicy
	return dto
}

func QuotaPolicyFromDTO(dto *QuotaPolicyDTO) domain.QuotaPolicy {
	if dto == nil {
		return nil
	}
	return domain.NewQuotaPolicy(dto.RRealmID, dto.RResourceType, dto.RMaxCount)
}

// ResourceTypeQuotaCheck is the JSON:API type for the quota-check operation.
const ResourceTypeQuotaCheck resource.Type = "quotaChecks"

// QuotaCheckRequestDTO asks whether `current` usage is under the realm's cap.
type QuotaCheckRequestDTO struct {
	resource.RestDTO

	RRealmID      string `jsonapi:"attr,realmId"`
	RResourceType string `jsonapi:"attr,resourceType"`
	RCurrent      int    `jsonapi:"attr,current"`
}

// QuotaCheckDTO is the decision.
type QuotaCheckDTO struct {
	resource.RestDTO

	RAllowed bool `jsonapi:"attr,allowed"`
}

func QuotaCheckToDTO(allowed bool) *QuotaCheckDTO {
	dto := &QuotaCheckDTO{RAllowed: allowed}
	dto.RType = ResourceTypeQuotaCheck
	return dto
}
