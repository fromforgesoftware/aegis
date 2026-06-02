package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeBinding is the JSON:API type for /api/bindings.
const ResourceTypeBinding resource.Type = "bindings"

// BindingDTO is the wire shape for an ACL binding: subject → role → resource.
type BindingDTO struct {
	resource.RestDTO

	RResourceID  string     `jsonapi:"attr,resourceId,omitempty"`
	RRoleID      string     `jsonapi:"attr,roleId,omitempty"`
	RSubjectType string     `jsonapi:"attr,subjectType,omitempty"`
	RSubjectID   string     `jsonapi:"attr,subjectId,omitempty"`
	RExpiresAt   *time.Time `jsonapi:"attr,expiresAt,omitempty"`
}

func BindingToDTO(b domain.Binding) *BindingDTO {
	if b == nil {
		return nil
	}
	dto := &BindingDTO{
		RestDTO:      resource.ToRestDTO(b),
		RResourceID:  b.ResourceID(),
		RRoleID:      b.RoleID(),
		RSubjectType: string(b.SubjectType()),
		RSubjectID:   b.SubjectID(),
		RExpiresAt:   b.ExpiresAt(),
	}
	dto.RType = ResourceTypeBinding
	return dto
}

func BindingFromDTO(dto *BindingDTO) domain.Binding {
	if dto == nil {
		return nil
	}
	return domain.NewBinding(dto.RResourceID, dto.RRoleID,
		domain.SubjectType(dto.RSubjectType), dto.RSubjectID,
		domain.WithBindingExpiresAt(dto.RExpiresAt))
}
