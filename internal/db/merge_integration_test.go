//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestMergeTransfers verifies the conflict-safe transfers against a real
// Postgres: external ids move wholesale; shared group memberships and
// duplicate bindings collapse onto the target without violating the unique
// constraints; the source is left with nothing.
func TestMergeTransfers(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "merge-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	authx := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(),
		gormdb.NewTransactioner(client, logger.New()))

	src, err := authx.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "src@x.com", Password: "correct horse battery staple"})
	require.NoError(t, err)
	dst, err := authx.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "dst@x.com", Password: "correct horse battery staple"})
	require.NoError(t, err)

	// A group both accounts belong to (shared) and one only the source has.
	sharedSet := insertGroup(t, client, realmID, "shared")
	srcOnlySet := insertGroup(t, client, realmID, "src-only")
	insertMember(t, client, sharedSet, src.ID())
	insertMember(t, client, sharedSet, dst.ID())
	insertMember(t, client, srcOnlySet, src.ID())

	// External ids on the source only.
	insertExternalID(t, client, src.ID(), "OAUTH_GOOGLE", "google-123")

	mergeRepo := db.NewAccountMergeRepository(client)

	ids, err := mergeRepo.TransferExternalIDs(ctx, src.ID(), dst.ID())
	require.NoError(t, err)
	assert.Equal(t, int64(1), ids)

	members, err := mergeRepo.TransferMemberships(ctx, src.ID(), dst.ID())
	require.NoError(t, err)
	assert.Equal(t, int64(1), members, "only the src-only set moves; the shared one is skipped")

	// The source has no memberships left; the target is in both sets.
	assert.Equal(t, 0, countMemberships(t, client, src.ID()))
	assert.Equal(t, 2, countMemberships(t, client, dst.ID()))

	require.NoError(t, mergeRepo.SoftDeleteSource(ctx, src.ID()))
	require.NoError(t, mergeRepo.RecordMergeEvent(ctx, app.AccountMergeEvent{
		SourceID: src.ID(), TargetID: dst.ID(), RealmID: realmID,
		Summary: app.MergeSummary{ExternalIDs: ids, Memberships: members},
	}))

	var deletedAt *string
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT deleted_at FROM aegis.account WHERE id = ?`, src.ID()).Scan(&deletedAt).Error)
	assert.NotNil(t, deletedAt, "source is tombstoned")

	var events int
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM aegis.account_merge_event WHERE source_id = ? AND target_id = ?`, src.ID(), dst.ID()).
		Scan(&events).Error)
	assert.Equal(t, 1, events)
}

func insertGroup(t *testing.T, client *gormdb.DBClient, realmID, name string) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, client.WithContext(context.Background()).
		Exec(`INSERT INTO aegis.actor_set (id, realm_id, name) VALUES (?, ?, ?)`, id, realmID, name).Error)
	return id
}

func insertMember(t *testing.T, client *gormdb.DBClient, setID, accountID string) {
	t.Helper()
	require.NoError(t, client.WithContext(context.Background()).
		Exec(`INSERT INTO aegis.actor_set_member (actor_set_id, account_id) VALUES (?, ?)`, setID, accountID).Error)
}

func insertExternalID(t *testing.T, client *gormdb.DBClient, accountID, kind, externalID string) {
	t.Helper()
	require.NoError(t, client.WithContext(context.Background()).
		Exec(`INSERT INTO aegis.account_external_id (account_id, kind, external_id) VALUES (?, ?, ?)`, accountID, kind, externalID).Error)
}

func countMemberships(t *testing.T, client *gormdb.DBClient, accountID string) int {
	t.Helper()
	var n int
	require.NoError(t, client.WithContext(context.Background()).
		Raw(`SELECT COUNT(*) FROM aegis.actor_set_member WHERE account_id = ?`, accountID).Scan(&n).Error)
	return n
}
