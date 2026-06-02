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

func TestGroupMemberRepository_MembershipRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "group-realm").Error)
	alice, bob := uuid.NewString(), uuid.NewString()
	for _, id := range []string{alice, bob} {
		require.NoError(t, client.WithContext(ctx).
			Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, id, realmID).Error)
	}

	sets, err := db.NewGroupRepository(client)
	require.NoError(t, err)
	members, err := db.NewGroupMemberRepository(client)
	require.NoError(t, err)

	group, err := sets.Create(ctx, domain.NewGroup(realmID, "editors"))
	require.NoError(t, err)
	require.NotEmpty(t, group.ID())

	require.NoError(t, members.CreateMany(ctx, group.ID(), []string{alice, bob}))
	got, err := members.ListAccountIDs(ctx, group.ID())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{alice, bob}, got)

	// Atomic overwrite: replacing the membership drops the prior rows.
	require.NoError(t, members.DeleteByGroup(ctx, group.ID()))
	require.NoError(t, members.CreateMany(ctx, group.ID(), []string{alice}))
	got, err = members.ListAccountIDs(ctx, group.ID())
	require.NoError(t, err)
	assert.Equal(t, []string{alice}, got)

	// Deleting the group cascades its membership away.
	require.NoError(t, client.WithContext(ctx).
		Exec(`DELETE FROM aegis.actor_set WHERE id = ?`, group.ID()).Error)
	got, err = members.ListAccountIDs(ctx, group.ID())
	require.NoError(t, err)
	assert.Empty(t, got)
}
