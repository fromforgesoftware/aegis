//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestExpiringGrants_SweepDropsExpired proves a future-dated binding is live,
// an already-expired binding is excluded by the projection, and the sweeper
// hard-deletes the expired row.
func TestExpiringGrants_SweepDropsExpired(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "exp-realm").Error)
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
	version := db.NewAuthzVersionRepository(client)
	reader := db.NewEffectiveAuthorizationRepository(client)
	projection := db.NewAuthorizationProjectionRepository(client)
	resolver := app.NewRoleResolver(rolePerms, roleComp, permInheritance, roleEff, persistencetest.NewTransactioner())
	authz := app.NewAuthorizationUsecase(projection, reader, resolver, version)
	sweeper := app.NewGrantSweeper(bindings, authz)

	_, err = permissions.Create(ctx, domain.NewPermission("doc.read", "doc", "read"))
	require.NoError(t, err)
	viewer, err := roles.Create(ctx, domain.NewRole(realmID, "viewer", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, viewer.ID(), []string{"doc.read"}))

	live, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc"))
	require.NoError(t, err)
	expiring, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc"))
	require.NoError(t, err)

	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	_, err = bindings.Create(ctx, domain.NewBinding(live.ID(), viewer.ID(), domain.SubjectTypeAccount, account,
		domain.WithBindingExpiresAt(&future)))
	require.NoError(t, err)
	expiredBind, err := bindings.Create(ctx, domain.NewBinding(expiring.ID(), viewer.ID(), domain.SubjectTypeAccount, account,
		domain.WithBindingExpiresAt(&past)))
	require.NoError(t, err)

	require.NoError(t, authz.Refresh(ctx))

	// The future-dated grant is live; the already-expired one is filtered out.
	liveAllowed, err := reader.Exists(ctx, account, live.ID(), "doc.read")
	require.NoError(t, err)
	assert.True(t, liveAllowed)
	expiredAllowed, err := reader.Exists(ctx, account, expiring.ID(), "doc.read")
	require.NoError(t, err)
	assert.False(t, expiredAllowed, "expired grant excluded by the projection")

	// The sweeper removes exactly the expired binding.
	removed, err := sweeper.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)

	_, err = bindings.Get(ctx, byID(expiredBind.ID()))
	require.Error(t, err, "expired binding hard-deleted")

	// The live grant survives the sweep.
	liveAllowed, err = reader.Exists(ctx, account, live.ID(), "doc.read")
	require.NoError(t, err)
	assert.True(t, liveAllowed)
}
