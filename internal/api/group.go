package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeGroup is the JSON:API type for /api/groups.
const ResourceTypeGroup resource.Type = "groups"

// GroupDTO is the wire shape for a group; members are managed at the
// /api/groups/{id}/members sub-resource, not via the group's attributes.
type GroupDTO struct {
	resource.RestDTO

	RRealmID     string `jsonapi:"attr,realmId,omitempty"`
	RName        string `jsonapi:"attr,name,omitempty"`
	RDescription string `jsonapi:"attr,description,omitempty"`

	ROrganization *resource.RelationshipDTO `jsonapi:"rel,organization,omitempty"`
}

func GroupToDTO(a domain.Group) *GroupDTO {
	if a == nil {
		return nil
	}
	dto := &GroupDTO{
		RestDTO:       resource.ToRestDTO(a),
		RRealmID:      a.RealmID(),
		RName:         a.Name(),
		RDescription:  a.Description(),
		ROrganization: resource.RelationshipToDTO(resource.RelFromIdentifier(a.Organization())),
	}
	dto.RType = ResourceTypeGroup
	return dto
}

func GroupFromDTO(dto *GroupDTO) domain.Group {
	if dto == nil {
		return nil
	}
	opts := []domain.GroupOption{domain.WithGroupDescription(dto.RDescription)}
	if dto.ROrganization != nil {
		opts = append(opts, domain.WithGroupOrganizationID(dto.ROrganization.ID()))
	}
	return domain.NewGroup(dto.RRealmID, dto.RName, opts...)
}
