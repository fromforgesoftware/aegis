package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

func validCatalog() domain.CatalogDocument {
	return domain.CatalogDocument{
		ResourceType: "strategies",
		Permissions: map[string]domain.CatalogPermissionSpec{
			"read":   {},
			"write":  {Implies: []string{"read"}},
			"delete": {},
		},
		Roles: map[string]domain.CatalogRoleSpec{
			"reader": {Permissions: []string{"strategies.read"}},
			"editor": {Composes: []string{"reader"}, Permissions: []string{"write"}},
			"admin":  {Composes: []string{"strategies.editor"}, Permissions: []string{"delete"}},
		},
	}
}

func TestCatalogValidate_OK(t *testing.T) {
	require.NoError(t, validCatalog().Validate())
}

func TestCatalogValidate_SlugDerivation(t *testing.T) {
	d := validCatalog()
	assert.Equal(t, "strategies.read", d.PermissionID("read"))
	assert.Equal(t, "strategies.admin", d.RoleID("admin"))
	// References normalize whether bare or fully qualified.
	assert.Equal(t, "read", d.Normalize("strategies.read"))
	assert.Equal(t, "read", d.Normalize("read"))
}

func TestCatalogValidate_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.CatalogDocument)
		want   string
	}{
		{"empty type", func(d *domain.CatalogDocument) { d.ResourceType = "" }, "resourceType is required"},
		{"dotted type", func(d *domain.CatalogDocument) { d.ResourceType = "a.b" }, "cannot contain"},
		{"no permissions", func(d *domain.CatalogDocument) { d.Permissions = nil }, "at least one permission"},
		{"dotted verb", func(d *domain.CatalogDocument) {
			d.Permissions["a.b"] = domain.CatalogPermissionSpec{}
		}, "invalid permission verb"},
		{"implies unknown", func(d *domain.CatalogDocument) {
			d.Permissions["write"] = domain.CatalogPermissionSpec{Implies: []string{"nope"}}
		}, "unknown permission"},
		{"implies self", func(d *domain.CatalogDocument) {
			d.Permissions["write"] = domain.CatalogPermissionSpec{Implies: []string{"strategies.write"}}
		}, "implies itself"},
		{"empty role", func(d *domain.CatalogDocument) {
			d.Roles["ghost"] = domain.CatalogRoleSpec{}
		}, "grants nothing"},
		{"role unknown permission", func(d *domain.CatalogDocument) {
			d.Roles["reader"] = domain.CatalogRoleSpec{Permissions: []string{"nope"}}
		}, "unknown permission"},
		{"composes unknown", func(d *domain.CatalogDocument) {
			d.Roles["editor"] = domain.CatalogRoleSpec{Composes: []string{"ghost"}}
		}, "composes unknown role"},
		{"composes self", func(d *domain.CatalogDocument) {
			d.Roles["editor"] = domain.CatalogRoleSpec{Composes: []string{"editor"}}
		}, "composes itself"},
		{"role collides with verb", func(d *domain.CatalogDocument) {
			d.Roles["read"] = domain.CatalogRoleSpec{Permissions: []string{"read"}}
		}, "collides with the permission verb"},
		{"invalid force entry", func(d *domain.CatalogDocument) {
			d.Force = []string{"other.type.admin"}
		}, "invalid force entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validCatalog()
			tc.mutate(&d)
			err := d.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestCatalogForces(t *testing.T) {
	d := validCatalog()
	d.Force = []string{"admin", "strategies.delete"}
	require.NoError(t, d.Validate())
	assert.True(t, d.Forces("strategies.admin"))
	assert.True(t, d.Forces("strategies.delete"))
	assert.False(t, d.Forces("strategies.reader"))
}

func TestCatalogValidate_ImplicationCycle(t *testing.T) {
	d := validCatalog()
	d.Permissions["read"] = domain.CatalogPermissionSpec{Implies: []string{"write"}}
	// write already implies read → cycle.
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "implication cycle")
}

func TestCatalogValidate_CompositionCycle(t *testing.T) {
	d := validCatalog()
	d.Roles["reader"] = domain.CatalogRoleSpec{Composes: []string{"admin"}, Permissions: []string{"read"}}
	// admin composes editor composes reader → cycle.
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "composition cycle")
}
