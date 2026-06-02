package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeMembership is the JSON:API type for an organization member —
// the readable view of an account↔role binding on the org's anchor resource.
const ResourceTypeMembership resource.Type = "memberships"

// MembershipDTO presents an org member as account + role under an organization.
// It is the friendly projection of the underlying ACL binding (whose id it
// carries); the generic Binding remains the low-level primitive.
type MembershipDTO struct {
	resource.RestDTO

	RAccount      *resource.RelationshipDTO `jsonapi:"rel,account,omitempty"`
	RRole         *resource.RelationshipDTO `jsonapi:"rel,role,omitempty"`
	ROrganization *resource.RelationshipDTO `jsonapi:"rel,organization,omitempty"`
}

// MembershipToDTO projects an account binding on an org's anchor into a
// Membership. orgID is the organization the members were listed under.
func MembershipToDTO(b domain.Binding, orgID string) *MembershipDTO {
	if b == nil {
		return nil
	}
	dto := &MembershipDTO{
		RestDTO:       resource.ToRestDTO(b),
		RAccount:      resource.RelationshipToDTO(resource.RelFromIDAndType(b.SubjectID(), domain.ResourceTypeAccount)),
		RRole:         resource.RelationshipToDTO(resource.RelFromIDAndType(b.RoleID(), domain.ResourceTypeRole)),
		ROrganization: resource.RelationshipToDTO(resource.RelFromIDAndType(orgID, ResourceTypeOrganization)),
	}
	dto.RType = ResourceTypeMembership
	return dto
}
