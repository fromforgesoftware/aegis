package db

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// TestOrganizationToEntity_SettingsNeverNil guards the org bootstrap regression:
// settings is jsonb NOT NULL DEFAULT '{}', but gorm lists the column in the
// INSERT, so a nil map would bind an explicit NULL and violate the constraint
// (the DEFAULT only applies when the column is omitted). The mapping must
// always produce valid jsonb.
func TestOrganizationToEntity_SettingsNeverNil(t *testing.T) {
	// No settings supplied (the bootstrap path) → empty object, never nil.
	e := organizationToEntity(domain.NewOrganization("realm-1", "Acme", "acme"))
	assert.Equal(t, []byte("{}"), e.ESettings)

	// Settings supplied → marshalled through.
	e2 := organizationToEntity(domain.NewOrganization("realm-1", "Acme", "acme",
		domain.WithOrganizationSettings(map[string]any{"k": "v"})))
	assert.JSONEq(t, `{"k":"v"}`, string(e2.ESettings))
}
