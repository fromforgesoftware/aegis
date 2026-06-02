package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeInvitation is the JSON:API type for /api/invitations.
const ResourceTypeInvitation resource.Type = "invitations"

// InvitationDTO is the wire shape for an invitation; the token is delivered
// out-of-band and never serialised.
type InvitationDTO struct {
	resource.RestDTO

	RRealmID    string    `jsonapi:"attr,realmId,omitempty"`
	REmail      string    `jsonapi:"attr,email,omitempty"`
	RInvitedBy  string    `jsonapi:"attr,invitedBy,omitempty"`
	RRoleID     string    `jsonapi:"attr,roleId,omitempty"`
	RResourceID string    `jsonapi:"attr,resourceId,omitempty"`
	RStatus     string    `jsonapi:"attr,status,omitempty"`
	RExpiresAt  time.Time `jsonapi:"attr,expiresAt,omitempty"`
}

func InvitationToDTO(i domain.Invitation) *InvitationDTO {
	if i == nil {
		return nil
	}
	dto := &InvitationDTO{
		RestDTO:     resource.ToRestDTO(i),
		RRealmID:    i.RealmID(),
		REmail:      i.Email(),
		RInvitedBy:  i.InvitedBy(),
		RRoleID:     i.RoleID(),
		RResourceID: i.ResourceID(),
		RStatus:     string(i.Status()),
		RExpiresAt:  i.ExpiresAt(),
	}
	dto.RType = ResourceTypeInvitation
	return dto
}

func InvitationFromDTO(dto *InvitationDTO) domain.Invitation {
	if dto == nil {
		return nil
	}
	return domain.NewInvitation(dto.RRealmID, dto.REmail, dto.RExpiresAt,
		domain.WithInvitationInvitedBy(dto.RInvitedBy),
		domain.WithInvitationRoleID(dto.RRoleID),
		domain.WithInvitationResourceID(dto.RResourceID),
	)
}

// InvitationAcceptRequestDTO is the body for accepting an invitation.
type InvitationAcceptRequestDTO struct {
	resource.RestDTO

	RToken     string `jsonapi:"attr,token"`
	RAccountID string `jsonapi:"attr,accountId"`
}
