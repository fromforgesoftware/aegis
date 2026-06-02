package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeInvitation is the JSON:API type for /api/invitations.
const ResourceTypeInvitation resource.Type = "invitations"

// InvitationStatus tracks an invite's lifecycle.
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "PENDING"
	InvitationStatusAccepted InvitationStatus = "ACCEPTED"
	InvitationStatusExpired  InvitationStatus = "EXPIRED"
	InvitationStatusRevoked  InvitationStatus = "REVOKED"
)

// Invitation is an emailed offer of a pre-assigned role on a resource; on
// accept the accepting account is bound to (role, resource).
type Invitation interface {
	resource.Resource
	RealmID() string
	Email() string
	InvitedBy() string
	RoleID() string
	ResourceID() string
	TokenHash() string
	Status() InvitationStatus
	ExpiresAt() time.Time
}

type invitation struct {
	resource.Resource

	realmID    string
	email      string
	invitedBy  string
	roleID     string
	resourceID string
	tokenHash  string
	status     InvitationStatus
	expiresAt  time.Time
}

type InvitationOption func(*invitation)

func WithInvitationID(id string) InvitationOption {
	return func(i *invitation) { i.Resource = resource.Update(i.Resource, resource.WithID(id)) }
}
func WithInvitationInvitedBy(id string) InvitationOption {
	return func(i *invitation) { i.invitedBy = id }
}
func WithInvitationRoleID(id string) InvitationOption {
	return func(i *invitation) { i.roleID = id }
}
func WithInvitationResourceID(id string) InvitationOption {
	return func(i *invitation) { i.resourceID = id }
}
func WithInvitationTokenHash(h string) InvitationOption {
	return func(i *invitation) { i.tokenHash = h }
}
func WithInvitationStatus(s InvitationStatus) InvitationOption {
	return func(i *invitation) { i.status = s }
}

// NewInvitation builds an invitation; realmID/email/expiresAt are mandatory,
// status defaults to PENDING.
func NewInvitation(realmID, email string, expiresAt time.Time, opts ...InvitationOption) Invitation {
	i := &invitation{
		Resource:  resource.New(resource.WithType(ResourceTypeInvitation)),
		realmID:   realmID,
		email:     email,
		status:    InvitationStatusPending,
		expiresAt: expiresAt,
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

func (i *invitation) RealmID() string          { return i.realmID }
func (i *invitation) Email() string            { return i.email }
func (i *invitation) InvitedBy() string        { return i.invitedBy }
func (i *invitation) RoleID() string           { return i.roleID }
func (i *invitation) ResourceID() string       { return i.resourceID }
func (i *invitation) TokenHash() string        { return i.tokenHash }
func (i *invitation) Status() InvitationStatus { return i.status }
func (i *invitation) ExpiresAt() time.Time     { return i.expiresAt }
