//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestResolver_CompositionAndInheritance drives the real resolver over Postgres
// and asserts a composite role's union and a permission-inheritance chain both
// land in the projection that Check reads.
func TestResolver_CompositionAndInheritance(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "compose-realm").Error)
	account := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, account, realmID).Error)

	permissions, err := db.NewPermissionRepository(client)
	require.NoError(t, err)
	permInheritance, err := db.NewPermissionInheritanceRepository(client)
	require.NoError(t, err)
	roles, err := db.NewRoleRepository(client)
	require.NoError(t, err)
	rolePerms, err := db.NewRolePermissionRepository(client)
	require.NoError(t, err)
	roleComp, err := db.NewRoleCompositionRepository(client)
	require.NoError(t, err)
	roleEff, err := db.NewRoleEffectivePermissionRepository(client)
	require.NoError(t, err)
	resources, err := db.NewAuthzResourceRepository(client)
	require.NoError(t, err)
	bindings, err := db.NewBindingRepository(client)
	require.NoError(t, err)
	projection := db.NewAuthorizationProjectionRepository(client)
	reader := db.NewEffectiveAuthorizationRepository(client)

	for _, p := range []string{"read", "write", "admin"} {
		_, err = permissions.Create(ctx, domain.NewPermission("doc."+p, "doc", p))
		require.NoError(t, err)
	}
	// doc.admin implies doc.write implies doc.read.
	require.NoError(t, permInheritance.CreateMany(ctx, "doc.admin", []string{"doc.write"}))
	require.NoError(t, permInheritance.CreateMany(ctx, "doc.write", []string{"doc.read"}))

	viewer, err := roles.Create(ctx, domain.NewRole(realmID, "viewer", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, viewer.ID(), []string{"doc.read"}))
	writer, err := roles.Create(ctx, domain.NewRole(realmID, "writer", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, writer.ID(), []string{"doc.write"}))
	super, err := roles.Create(ctx, domain.NewRole(realmID, "super", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, super.ID(), []string{"doc.admin"}))

	// editor is a composite: viewer UNION writer, no direct grants.
	editor, err := roles.Create(ctx, domain.NewRole(realmID, "editor", "doc"))
	require.NoError(t, err)
	require.NoError(t, roleComp.CreateMany(ctx, editor.ID(), []domain.RoleComponent{
		{ComponentRoleID: viewer.ID(), Operator: domain.CompositionUnion, Ordinal: 0},
		{ComponentRoleID: writer.ID(), Operator: domain.CompositionUnion, Ordinal: 1},
	}))

	doc1, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc"))
	require.NoError(t, err)
	doc2, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc"))
	require.NoError(t, err)
	_, err = bindings.Create(ctx, domain.NewBinding(doc1.ID(), editor.ID(), domain.SubjectTypeAccount, account))
	require.NoError(t, err)
	_, err = bindings.Create(ctx, domain.NewBinding(doc2.ID(), super.ID(), domain.SubjectTypeAccount, account))
	require.NoError(t, err)

	resolver := app.NewRoleResolver(rolePerms, roleComp, permInheritance, roleEff, persistencetest.NewTransactioner())
	require.NoError(t, resolver.Resolve(ctx))
	require.NoError(t, projection.Refresh(ctx))

	// Composite editor on doc1 grants viewer's read + writer's write, not admin.
	for perm, want := range map[string]bool{"doc.read": true, "doc.write": true, "doc.admin": false} {
		got, err := reader.Exists(ctx, account, doc1.ID(), perm)
		require.NoError(t, err)
		assert.Equal(t, want, got, "editor/%s on doc1", perm)
	}
	// super on doc2 holds doc.admin, which inherits down to write and read.
	for _, perm := range []string{"doc.admin", "doc.write", "doc.read"} {
		got, err := reader.Exists(ctx, account, doc2.ID(), perm)
		require.NoError(t, err)
		assert.True(t, got, "super/%s on doc2 via inheritance", perm)
	}
}
