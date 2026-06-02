//go:build integration

package db_test

import (
	"context"
	"testing"

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

func TestAuthzResourceRepository_HierarchyRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "authz-realm").Error)

	repo, err := db.NewAuthzResourceRepository(client)
	require.NoError(t, err)

	workspace, err := repo.Create(ctx, domain.NewAuthzResource(realmID, "workspace"))
	require.NoError(t, err)
	require.NotEmpty(t, workspace.ID())

	doc, err := repo.Create(ctx, domain.NewAuthzResource(realmID, "doc",
		domain.WithAuthzResourceParentID(workspace.ID()),
	))
	require.NoError(t, err)
	assert.Equal(t, workspace.ID(), doc.ParentID(), "parent_id survives the round-trip")

	got, err := repo.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, doc.ID())))
	require.NoError(t, err)
	assert.Equal(t, workspace.ID(), got.ParentID())

	// Filtering by parent_id lists the workspace's direct children.
	children, err := repo.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.ParentID, workspace.ID()),
	))
	require.NoError(t, err)
	require.Len(t, children.Results(), 1)
	assert.Equal(t, doc.ID(), children.Results()[0].ID())

	// Cascading delete: dropping the workspace also clears the doc beneath it.
	require.NoError(t, client.WithContext(ctx).
		Exec(`DELETE FROM aegis.resource WHERE id = ?`, workspace.ID()).Error)
	_, err = repo.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, doc.ID())))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}
