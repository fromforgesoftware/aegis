//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestEffectiveAuthorizationReader_OverProjection(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "reader-realm").Error)
	account := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, account, realmID).Error)

	permissions, err := db.NewPermissionRepository(client)
	require.NoError(t, err)
	roles, err := db.NewRoleRepository(client)
	require.NoError(t, err)
	rolePerms, err := db.NewRolePermissionRepository(client)
	require.NoError(t, err)
	resources, err := db.NewAuthzResourceRepository(client)
	require.NoError(t, err)
	bindings, err := db.NewBindingRepository(client)
	require.NoError(t, err)
	roleEff, err := db.NewRoleEffectivePermissionRepository(client)
	require.NoError(t, err)
	projection := db.NewAuthorizationProjectionRepository(client)
	reader := db.NewEffectiveAuthorizationRepository(client)

	_, err = permissions.Create(ctx, domain.NewPermission("doc.read", "doc", "read"))
	require.NoError(t, err)
	_, err = permissions.Create(ctx, domain.NewPermission("doc.write", "doc", "write"))
	require.NoError(t, err)

	editor, err := roles.Create(ctx, domain.NewRole(realmID, "editor", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, editor.ID(), []string{"doc.read", "doc.write"}))
	require.NoError(t, roleEff.CreateMany(ctx, editor.ID(), []string{"doc.read", "doc.write"}))

	workspace, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "workspace"))
	require.NoError(t, err)
	doc, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc",
		domain.WithAuthzResourceParentID(workspace.ID())))
	require.NoError(t, err)

	// editor on the workspace inherits down to the doc.
	_, err = bindings.Create(ctx, domain.NewBinding(workspace.ID(), editor.ID(), domain.SubjectTypeAccount, account))
	require.NoError(t, err)
	require.NoError(t, projection.Refresh(ctx))

	allowed, err := reader.Exists(ctx, account, doc.ID(), "doc.read")
	require.NoError(t, err)
	assert.True(t, allowed, "inherited grant on the doc")

	denied, err := reader.Exists(ctx, account, doc.ID(), "doc.delete")
	require.NoError(t, err)
	assert.False(t, denied, "permission the role never had")

	accessible, err := reader.ListResourceIDs(ctx, account, "doc.read")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{workspace.ID(), doc.ID()}, accessible)

	pairs, err := reader.AllowedPairs(ctx, account, []domain.PermissionCheck{
		{ResourceID: doc.ID(), PermissionID: "doc.write"},
		{ResourceID: doc.ID(), PermissionID: "doc.delete"},
	})
	require.NoError(t, err)
	assert.Equal(t, []domain.PermissionCheck{{ResourceID: doc.ID(), PermissionID: "doc.write"}}, pairs)
}
