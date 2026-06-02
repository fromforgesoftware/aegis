package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeRole is the JSON:API type for /api/roles.
const ResourceTypeRole resource.Type = "roles"

// RoleDTO is the wire shape for a role; permissions are managed at the
// /api/roles/{id}/permissions sub-resource, not via the role's attributes.
type RoleDTO struct {
	resource.RestDTO

	RRealmID      string `jsonapi:"attr,realmId,omitempty"`
	RName         string `jsonapi:"attr,name,omitempty"`
	RResourceType string `jsonapi:"attr,resourceType,omitempty"`
	RDescription  string `jsonapi:"attr,description,omitempty"`
	RKind         string `jsonapi:"attr,kind,omitempty"`
}

func RoleToDTO(r domain.Role) *RoleDTO {
	if r == nil {
		return nil
	}
	dto := &RoleDTO{
		RestDTO:       resource.ToRestDTO(r),
		RRealmID:      r.RealmID(),
		RName:         r.Name(),
		RResourceType: r.ResourceType(),
		RDescription:  r.Description(),
		RKind:         string(r.Kind()),
	}
	dto.RType = ResourceTypeRole
	return dto
}

func RoleFromDTO(dto *RoleDTO) domain.Role {
	if dto == nil {
		return nil
	}
	kind := domain.RoleKind(dto.RKind)
	if kind == "" {
		kind = domain.RoleKindCustom
	}
	return domain.NewRole(dto.RRealmID, dto.RName, dto.RResourceType,
		domain.WithRoleDescription(dto.RDescription),
		domain.WithRoleKind(kind),
	)
}
