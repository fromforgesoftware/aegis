//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

type effectiveAuth struct {
	AccountID    string `gorm:"column:account_id"`
	PermissionID string `gorm:"column:permission_id"`
	ResourceID   string `gorm:"column:resource_id"`
}

// TestAuthorizationProjection_Closure builds a realistic grant graph and
// asserts the materialised view flattens it correctly: the resource hierarchy
// inherits a parent grant down to its child, and a group binding fans out to
// the group's members.
func TestAuthorizationProjection_Closure(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "proj-realm").Error)
	alice, bob := uuid.NewString(), uuid.NewString()
	for _, id := range []string{alice, bob} {
		require.NoError(t, client.WithContext(ctx).
			Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, id, realmID).Error)
	}

	permissions, err := db.NewPermissionRepository(client)
	require.NoError(t, err)
	roles, err := db.NewRoleRepository(client)
	require.NoError(t, err)
	rolePerms, err := db.NewRolePermissionRepository(client)
	require.NoError(t, err)
	resources, err := db.NewAuthzResourceRepository(client)
	require.NoError(t, err)
	sets, err := db.NewGroupRepository(client)
	require.NoError(t, err)
	members, err := db.NewGroupMemberRepository(client)
	require.NoError(t, err)
	bindings, err := db.NewBindingRepository(client)
	require.NoError(t, err)
	roleEff, err := db.NewRoleEffectivePermissionRepository(client)
	require.NoError(t, err)
	projection := db.NewAuthorizationProjectionRepository(client)

	_, err = permissions.Create(ctx, domain.NewPermission("ws.admin", "workspace", "admin"))
	require.NoError(t, err)
	_, err = permissions.Create(ctx, domain.NewPermission("doc.read", "doc", "read"))
	require.NoError(t, err)

	wsAdmin, err := roles.Create(ctx, domain.NewRole(realmID, "ws-admin", "workspace"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, wsAdmin.ID(), []string{"ws.admin"}))
	docEditor, err := roles.Create(ctx, domain.NewRole(realmID, "doc-editor", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, docEditor.ID(), []string{"doc.read"}))

	// The projection joins role_effective_permission (the resolver's output);
	// with no composition or inheritance, it equals the direct grants.
	require.NoError(t, roleEff.CreateMany(ctx, wsAdmin.ID(), []string{"ws.admin"}))
	require.NoError(t, roleEff.CreateMany(ctx, docEditor.ID(), []string{"doc.read"}))

	workspace, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "workspace"))
	require.NoError(t, err)
	doc, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc",
		domain.WithAuthzResourceParentID(workspace.ID())))
	require.NoError(t, err)

	team, err := sets.Create(ctx, domain.NewGroup(realmID, "team"))
	require.NoError(t, err)
	require.NoError(t, members.CreateMany(ctx, team.ID(), []string{bob}))

	// alice gets ws-admin on the workspace; it must inherit down to the doc.
	aliceBind, err := bindings.Create(ctx, domain.NewBinding(workspace.ID(), wsAdmin.ID(), domain.SubjectTypeAccount, alice))
	require.NoError(t, err)
	// the team (bob) gets doc-editor on the doc only.
	_, err = bindings.Create(ctx, domain.NewBinding(doc.ID(), docEditor.ID(), domain.SubjectTypeGroup, team.ID()))
	require.NoError(t, err)

	require.NoError(t, projection.Refresh(ctx))

	var rows []effectiveAuth
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT account_id, permission_id, resource_id FROM aegis.effective_authorizations`).
		Scan(&rows).Error)

	assert.ElementsMatch(t, []effectiveAuth{
		{AccountID: alice, PermissionID: "ws.admin", ResourceID: workspace.ID()},
		{AccountID: alice, PermissionID: "ws.admin", ResourceID: doc.ID()},
		{AccountID: bob, PermissionID: "doc.read", ResourceID: doc.ID()},
	}, rows)

	// Revoking the workspace grant (soft delete) and refreshing drops alice's
	// inherited rows — the projection filters deleted_at IS NULL.
	require.NoError(t, bindings.Delete(ctx, repository.DeleteTypeSoft, byID(aliceBind.ID())))
	require.NoError(t, projection.Refresh(ctx))

	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT account_id, permission_id, resource_id FROM aegis.effective_authorizations`).
		Scan(&rows).Error)
	assert.ElementsMatch(t, []effectiveAuth{
		{AccountID: bob, PermissionID: "doc.read", ResourceID: doc.ID()},
	}, rows)
}
