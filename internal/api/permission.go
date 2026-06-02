package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypePermission is the JSON:API type for /api/permissions.
const ResourceTypePermission resource.Type = "permissions"

// PermissionDTO is the wire shape for a permission catalog entry. The id is
// the slug ("doc.read") rather than a UUID.
type PermissionDTO struct {
	resource.RestDTO

	RResourceType string `jsonapi:"attr,resourceType,omitempty"`
	RVerb         string `jsonapi:"attr,verb,omitempty"`
	RDescription  string `jsonapi:"attr,description,omitempty"`
}

func PermissionToDTO(p domain.Permission) *PermissionDTO {
	if p == nil {
		return nil
	}
	dto := &PermissionDTO{
		RestDTO:       resource.ToRestDTO(p),
		RResourceType: p.ResourceType(),
		RVerb:         p.Verb(),
		RDescription:  p.Description(),
	}
	dto.RType = ResourceTypePermission
	return dto
}

func PermissionFromDTO(dto *PermissionDTO) domain.Permission {
	if dto == nil {
		return nil
	}
	return domain.NewPermission(dto.RID, dto.RResourceType, dto.RVerb,
		domain.WithPermissionDescription(dto.RDescription))
}
