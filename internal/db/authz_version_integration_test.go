//go:build integration

package db_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestAuthzVersion_ReadAfterWrite proves the version contract end to end: an
// authz write advances write_version (via trigger) past projection_version, a
// MinVersion read is rejected as stale until Refresh publishes, then the same
// read succeeds.
func TestAuthzVersion_ReadAfterWrite(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "ver-realm").Error)
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

	_, err = permissions.Create(ctx, domain.NewPermission("doc.read", "doc", "read"))
	require.NoError(t, err)
	viewer, err := roles.Create(ctx, domain.NewRole(realmID, "viewer", "doc"))
	require.NoError(t, err)
	require.NoError(t, rolePerms.CreateMany(ctx, viewer.ID(), []string{"doc.read"}))
	doc, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc"))
	require.NoError(t, err)
	_, err = bindings.Create(ctx, domain.NewBinding(doc.ID(), viewer.ID(), domain.SubjectTypeAccount, account))
	require.NoError(t, err)

	// The writes above advanced write_version past projection_version (still 0).
	write, projVersion, err := version.Versions(ctx)
	require.NoError(t, err)
	assert.Greater(t, write, int64(0))
	assert.Less(t, projVersion, write)

	// A read demanding the just-written version is stale before a refresh.
	_, err = authz.Check(ctx, account, doc.ID(), "doc.read", write)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodePreconditionFailed))

	require.NoError(t, authz.Refresh(ctx))

	// projection_version has caught up, so the same MinVersion read succeeds.
	_, published, err := version.Versions(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, published, write)

	allowed, err := authz.Check(ctx, account, doc.ID(), "doc.read", write)
	require.NoError(t, err)
	assert.True(t, allowed)
}
