//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// stubAuthz satisfies app.AuthorizationUsecase but only implements Refresh —
// the sole method catalog Apply calls. Any other call panics (embedded nil).
type stubAuthz struct{ app.AuthorizationUsecase }

func (stubAuthz) Refresh(context.Context) error { return nil }

// TestCatalogApply_ImpliedPermissionSortsEarlier is the regression guard for
// the FK-ordering bug: convergePermissions must create EVERY permission before
// inserting any implication edge, because an edge can point at a permission
// that sorts alphabetically later. Here "publish" (sorts before "read") implies
// "read"; inserting the publish→read edge before read exists violates the
// permission_inheritance FK. Apply must succeed and the edge must land.
func TestCatalogApply_ImpliedPermissionSortsEarlier(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	permissions, err := db.NewPermissionRepository(client)
	require.NoError(t, err)
	inheritance, err := db.NewPermissionInheritanceRepository(client)
	require.NoError(t, err)
	roles, err := db.NewRoleRepository(client)
	require.NoError(t, err)
	links, err := db.NewRolePermissionRepository(client)
	require.NoError(t, err)
	comps, err := db.NewRoleCompositionRepository(client)
	require.NoError(t, err)
	catalogs := db.NewCatalogRepository(client)

	uc := app.NewCatalogUsecase(permissions, inheritance, roles, links, comps, catalogs,
		stubAuthz{}, app.NoopAuditor{}, gormdb.NewTransactioner(client, logger.New()))

	doc := domain.CatalogDocument{
		ResourceType: "strategies",
		Permissions: map[string]domain.CatalogPermissionSpec{
			"create":  {Description: "create"},
			"read":    {Description: "read"},
			"update":  {Description: "update", Implies: []string{"read"}},
			"publish": {Description: "publish", Implies: []string{"read"}},
			"delete":  {Description: "delete"},
		},
		Roles: map[string]domain.CatalogRoleSpec{
			"editor": {Description: "editor", Permissions: []string{"update"}},
		},
	}

	require.NoError(t, uc.Apply(ctx, []domain.CatalogDocument{doc}))

	implied, err := inheritance.ListImpliedIDs(ctx, "strategies.publish")
	require.NoError(t, err)
	assert.Contains(t, implied, "strategies.read", "publish→read edge must be persisted")

	// Idempotent re-apply (the startup reconcile runs on every boot).
	require.NoError(t, uc.Apply(ctx, []domain.CatalogDocument{doc}))
}
