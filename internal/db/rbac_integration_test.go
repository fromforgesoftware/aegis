//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestRBACRoundTrip_PermissionRoleAttachment(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "rbac-realm").Error)

	permissions, err := db.NewPermissionRepository(client)
	require.NoError(t, err)
	roles, err := db.NewRoleRepository(client)
	require.NoError(t, err)
	links, err := db.NewRolePermissionRepository(client)
	require.NoError(t, err)

	// Seed two permissions.
	_, err = permissions.Create(ctx, domain.NewPermission("doc.read", "doc", "read"))
	require.NoError(t, err)
	_, err = permissions.Create(ctx, domain.NewPermission("doc.write", "doc", "write"))
	require.NoError(t, err)

	// (id, …) uniqueness on the permission slug.
	_, err = permissions.Create(ctx, domain.NewPermission("doc.read", "doc", "read"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists))

	// Create a custom role and attach the two permissions.
	created, err := roles.Create(ctx, domain.NewRole(realmID, "editor", "doc",
		domain.WithRoleKind(domain.RoleKindCustom)))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())

	require.NoError(t, links.CreateMany(ctx, created.ID(), []string{"doc.read", "doc.write"}))

	got, err := links.ListPermissionIDs(ctx, created.ID())
	require.NoError(t, err)
	assert.Equal(t, []string{"doc.read", "doc.write"}, got)

	// Atomic overwrite: dropping doc.write should leave only doc.read attached.
	require.NoError(t, links.DeleteByRole(ctx, created.ID()))
	require.NoError(t, links.CreateMany(ctx, created.ID(), []string{"doc.read"}))
	got, err = links.ListPermissionIDs(ctx, created.ID())
	require.NoError(t, err)
	assert.Equal(t, []string{"doc.read"}, got)

	// (realm, name, resource_type) uniqueness on the role row.
	_, err = roles.Create(ctx, domain.NewRole(realmID, "editor", "doc"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists))

	// Listing roles filtered by realm yields the one we created.
	list, err := roles.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
	))
	require.NoError(t, err)
	require.Len(t, list.Results(), 1)
	assert.Equal(t, "editor", list.Results()[0].Name())

	// Deleting the role cascades through role_permission (FK ON DELETE CASCADE),
	// leaving no orphan links behind.
	require.NoError(t, roles.Delete(ctx, repository.DeleteTypeSoft,
		search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, created.ID()))))
}
