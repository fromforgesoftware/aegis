package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeAuthzResource is the JSON:API type for /api/resources.
const ResourceTypeAuthzResource resource.Type = "resources"

// AuthzResourceDTO is the wire shape for an authz-registered resource.
type AuthzResourceDTO struct {
	resource.RestDTO

	RRealmID        string `jsonapi:"attr,realmId,omitempty"`
	RResourceType   string `jsonapi:"attr,type,omitempty"`
	ROwnerAccountID string `jsonapi:"attr,ownerAccountId,omitempty"`
	RParentID       string `jsonapi:"attr,parentId,omitempty"`
	RInheritVia     string `jsonapi:"attr,inheritVia,omitempty"`
	RVisibility     string `jsonapi:"attr,visibility,omitempty"`
}

func AuthzResourceToDTO(r domain.AuthzResource) *AuthzResourceDTO {
	if r == nil {
		return nil
	}
	dto := &AuthzResourceDTO{
		RestDTO:         resource.ToRestDTO(r),
		RRealmID:        r.RealmID(),
		RResourceType:   r.ResourceType(),
		ROwnerAccountID: r.OwnerAccountID(),
		RParentID:       r.ParentID(),
		RInheritVia:     r.InheritVia(),
		RVisibility:     string(r.Visibility()),
	}
	dto.RType = ResourceTypeAuthzResource
	return dto
}

func AuthzResourceFromDTO(dto *AuthzResourceDTO) domain.AuthzResource {
	if dto == nil {
		return nil
	}
	visibility := domain.Visibility(dto.RVisibility)
	if visibility == "" {
		visibility = domain.VisibilityPrivate
	}
	return domain.NewAuthzResource(dto.RRealmID, dto.RResourceType,
		domain.WithAuthzResourceOwnerAccountID(dto.ROwnerAccountID),
		domain.WithAuthzResourceParentID(dto.RParentID),
		domain.WithAuthzResourceInheritVia(dto.RInheritVia),
		domain.WithAuthzResourceVisibility(visibility),
	)
}
