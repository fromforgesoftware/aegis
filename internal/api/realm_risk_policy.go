package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

const ResourceTypeRealmRiskPolicy resource.Type = "realmRiskPolicies"

// RealmRiskPolicyDTO is the per-realm risk policy over the wire.
type RealmRiskPolicyDTO struct {
	resource.RestDTO

	RRealmID         string `jsonapi:"attr,realmId"`
	RNewIPWeight     int    `jsonapi:"attr,newIpWeight"`
	RNewDeviceWeight int    `jsonapi:"attr,newDeviceWeight"`
	RFailureWeight   int    `jsonapi:"attr,failureWeight"`
	RStepUpThreshold int    `jsonapi:"attr,stepUpThreshold"`
	RDenyThreshold   int    `jsonapi:"attr,denyThreshold"`
}

func RealmRiskPolicyToDTO(p domain.RealmRiskPolicy) *RealmRiskPolicyDTO {
	if p == nil {
		return nil
	}
	pol := p.Policy()
	dto := &RealmRiskPolicyDTO{
		RestDTO:          resource.ToRestDTO(p),
		RRealmID:         p.RealmID(),
		RNewIPWeight:     pol.NewIPWeight,
		RNewDeviceWeight: pol.NewDeviceWeight,
		RFailureWeight:   pol.FailureWeight,
		RStepUpThreshold: pol.StepUpThreshold,
		RDenyThreshold:   pol.DenyThreshold,
	}
	dto.RType = ResourceTypeRealmRiskPolicy
	dto.RID = p.RealmID()
	return dto
}

// RiskPolicyFromDTO extracts the realm id + policy weights from the request.
func RiskPolicyFromDTO(dto *RealmRiskPolicyDTO) (string, domain.RiskPolicy) {
	if dto == nil {
		return "", domain.RiskPolicy{}
	}
	return dto.RRealmID, domain.RiskPolicy{
		NewIPWeight:     dto.RNewIPWeight,
		NewDeviceWeight: dto.RNewDeviceWeight,
		FailureWeight:   dto.RFailureWeight,
		StepUpThreshold: dto.RStepUpThreshold,
		DenyThreshold:   dto.RDenyThreshold,
	}
}

// RiskPolicyValueToDTO renders a bare RiskPolicy (used by GET, which resolves
// to the default when no realm override exists) under the given realm id.
func RiskPolicyValueToDTO(realmID string, pol domain.RiskPolicy) *RealmRiskPolicyDTO {
	dto := &RealmRiskPolicyDTO{
		RRealmID:         realmID,
		RNewIPWeight:     pol.NewIPWeight,
		RNewDeviceWeight: pol.NewDeviceWeight,
		RFailureWeight:   pol.FailureWeight,
		RStepUpThreshold: pol.StepUpThreshold,
		RDenyThreshold:   pol.DenyThreshold,
	}
	dto.RType = ResourceTypeRealmRiskPolicy
	dto.RID = realmID
	return dto
}
