package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeBinding is the JSON:API type for /api/bindings.
const ResourceTypeBinding resource.Type = "bindings"

// SubjectType discriminates whether a binding's subject is a single account
// or a group whose members all inherit the grant.
type SubjectType string

const (
	SubjectTypeAccount SubjectType = "ACCOUNT"
	// SubjectTypeGroup's wire/DB value stays "ACTOR_SET" (the original enum).
	SubjectTypeGroup SubjectType = "ACTOR_SET"
)

func (s SubjectType) Valid() bool {
	switch s {
	case SubjectTypeAccount, SubjectTypeGroup:
		return true
	}
	return false
}

// Binding is an ACL grant: subject (account or group) → role → resource.
// The role's resource_type must match the bound resource's type, enforced at
// the usecase layer so a doc-role can't be granted on a workspace.
type Binding interface {
	resource.Resource
	ResourceID() string
	RoleID() string
	SubjectType() SubjectType
	SubjectID() string
	ExpiresAt() *time.Time
}

type binding struct {
	resource.Resource

	resourceID  string
	roleID      string
	subjectType SubjectType
	subjectID   string
	expiresAt   *time.Time
}

type BindingOption func(*binding)

func WithBindingID(id string) BindingOption {
	return func(b *binding) { b.Resource = resource.Update(b.Resource, resource.WithID(id)) }
}

// WithBindingExpiresAt makes the grant time-expiring; nil means it never
// expires.
func WithBindingExpiresAt(t *time.Time) BindingOption {
	return func(b *binding) { b.expiresAt = t }
}

// NewBinding builds an ACL binding aggregate; all four references are
// mandatory and validated by the usecase.
func NewBinding(resourceID, roleID string, subjectType SubjectType, subjectID string, opts ...BindingOption) Binding {
	b := &binding{
		Resource:    resource.New(resource.WithType(ResourceTypeBinding)),
		resourceID:  resourceID,
		roleID:      roleID,
		subjectType: subjectType,
		subjectID:   subjectID,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *binding) ResourceID() string       { return b.resourceID }
func (b *binding) RoleID() string           { return b.roleID }
func (b *binding) SubjectType() SubjectType { return b.subjectType }
func (b *binding) SubjectID() string        { return b.subjectID }
func (b *binding) ExpiresAt() *time.Time    { return b.expiresAt }
