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

func TestBindingRepository_GrantRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "acl-realm").Error)
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)

	resources, err := db.NewAuthzResourceRepository(client)
	require.NoError(t, err)
	roles, err := db.NewRoleRepository(client)
	require.NoError(t, err)
	bindings, err := db.NewBindingRepository(client)
	require.NoError(t, err)

	doc, err := resources.Create(ctx, domain.NewAuthzResource(realmID, "doc"))
	require.NoError(t, err)
	editor, err := roles.Create(ctx, domain.NewRole(realmID, "editor", "doc", domain.WithRoleKind(domain.RoleKindCustom)))
	require.NoError(t, err)

	bind, err := bindings.Create(ctx, domain.NewBinding(doc.ID(), editor.ID(), domain.SubjectTypeAccount, accountID))
	require.NoError(t, err)
	require.NotEmpty(t, bind.ID())

	got, err := bindings.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, bind.ID())))
	require.NoError(t, err)
	assert.Equal(t, doc.ID(), got.ResourceID())
	assert.Equal(t, editor.ID(), got.RoleID())
	assert.Equal(t, domain.SubjectTypeAccount, got.SubjectType())
	assert.Equal(t, accountID, got.SubjectID())

	// Listing by subject finds the account's grants.
	list, err := bindings.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.SubjectID, accountID),
	))
	require.NoError(t, err)
	require.Len(t, list.Results(), 1)

	// Deleting the resource cascades the binding away.
	require.NoError(t, client.WithContext(ctx).
		Exec(`DELETE FROM aegis.resource WHERE id = ?`, doc.ID()).Error)
	_, err = bindings.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, bind.ID())))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}
