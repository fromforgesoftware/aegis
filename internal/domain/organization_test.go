package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestNewOrganization_Defaults(t *testing.T) {
	o := domain.NewOrganization("realm-1", "Acme", "acme")

	assert.Equal(t, domain.ResourceTypeOrganization, o.Type())
	assert.Equal(t, "Acme", o.Name())
	assert.Equal(t, "acme", o.Slug())
	assert.Equal(t, domain.OrgStatusActive, o.Status())

	assert.Equal(t, "realm-1", o.Realm().ID())
	assert.Equal(t, domain.ResourceTypeRealm, o.Realm().Type())

	assert.Nil(t, o.AnchorResource(), "anchor unset until persisted")
	assert.Nil(t, o.Owner(), "owner optional")
}

func TestNewOrganization_Relationships(t *testing.T) {
	o := domain.NewOrganization("realm-1", "Acme", "acme",
		domain.WithOrganizationResourceID("res-1"),
		domain.WithOrganizationOwnerID("acc-1"),
		domain.WithOrganizationStatus(domain.OrgStatusSuspended),
		domain.WithOrganizationSettings(map[string]any{"theme": "dark"}),
	)

	if assert.NotNil(t, o.AnchorResource()) {
		assert.Equal(t, "res-1", o.AnchorResource().ID())
		assert.Equal(t, domain.ResourceTypeAuthzResource, o.AnchorResource().Type())
	}
	if assert.NotNil(t, o.Owner()) {
		assert.Equal(t, "acc-1", o.Owner().ID())
		assert.Equal(t, domain.ResourceTypeAccount, o.Owner().Type())
	}
	assert.Equal(t, domain.OrgStatusSuspended, o.Status())
	assert.Equal(t, "dark", o.Settings()["theme"])

	var _ resource.Resource = o
}

func TestOrgStatus_Valid(t *testing.T) {
	for _, s := range []domain.OrgStatus{domain.OrgStatusActive, domain.OrgStatusSuspended, domain.OrgStatusArchived} {
		assert.True(t, s.Valid(), string(s))
	}
	assert.False(t, domain.OrgStatus("bogus").Valid())
}
