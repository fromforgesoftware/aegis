package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeRealmACRPolicy is the JSON:API type for /api/realm-acr-policies.
const ResourceTypeRealmACRPolicy resource.Type = "realmAcrPolicies"

// RealmACRPolicyDTO is the wire shape for a realm's MFA / assurance policy.
type RealmACRPolicyDTO struct {
	resource.RestDTO

	RRealmID     string `jsonapi:"attr,realmId,omitempty"`
	RMFARequired bool   `jsonapi:"attr,mfaRequired"`
	RRequiredACR string `jsonapi:"attr,requiredAcr,omitempty"`
}

func RealmACRPolicyToDTO(p domain.RealmACRPolicy) *RealmACRPolicyDTO {
	if p == nil {
		return nil
	}
	dto := &RealmACRPolicyDTO{
		RestDTO:      resource.ToRestDTO(p),
		RRealmID:     p.RealmID(),
		RMFARequired: p.MFARequired(),
		RRequiredACR: p.RequiredACR(),
	}
	dto.RType = ResourceTypeRealmACRPolicy
	return dto
}

func RealmACRPolicyFromDTO(dto *RealmACRPolicyDTO) domain.RealmACRPolicy {
	if dto == nil {
		return nil
	}
	return domain.NewRealmACRPolicy(dto.RRealmID, dto.RMFARequired, dto.RRequiredACR)
}
