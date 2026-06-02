//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestModerationLifecycle exercises ban/unban + expiry sweep against a real
// Postgres: a timed ban writes the moderation columns, the sweeper restores
// only expired bans (permanent bans stay), and unban clears the state.
func TestModerationLifecycle(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "mod-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	uc := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(),
		gormdb.NewTransactioner(client, logger.New()))

	const password = "correct horse battery staple"
	temp, err := uc.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "temp@example.com", Password: password})
	require.NoError(t, err)
	perm, err := uc.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "perm@example.com", Password: password})
	require.NoError(t, err)

	// A timed ban writes status + banned_until + ban_reason.
	until := time.Now().Add(48 * time.Hour)
	require.NoError(t, accRepo.Ban(ctx, temp.ID(), &until, "spam"))
	assert.Equal(t, "BANNED", accountStatus(t, client, temp.ID()))

	var reason string
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT ban_reason FROM aegis.account WHERE id = ?`, temp.ID()).Scan(&reason).Error)
	assert.Equal(t, "spam", reason)

	// A permanent ban (nil until) is never restored by the sweeper.
	require.NoError(t, accRepo.Ban(ctx, perm.ID(), nil, "fraud"))

	// Sweeping with a now past the timed ban restores only the timed one.
	restored, err := accRepo.RestoreExpiredBans(ctx, until.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), restored)
	assert.Equal(t, "ENABLED", accountStatus(t, client, temp.ID()))
	assert.Equal(t, "BANNED", accountStatus(t, client, perm.ID()))

	// Unban lifts the permanent ban and clears the fields.
	require.NoError(t, accRepo.Unban(ctx, perm.ID()))
	assert.Equal(t, "ENABLED", accountStatus(t, client, perm.ID()))

	var bannedUntil, banReason *string
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT banned_until, ban_reason FROM aegis.account WHERE id = ?`, perm.ID()).
		Row().Scan(&bannedUntil, &banReason))
	assert.Nil(t, bannedUntil)
	assert.Nil(t, banReason)
}

func accountStatus(t *testing.T, client *gormdb.DBClient, accountID string) string {
	t.Helper()
	var status string
	require.NoError(t, client.WithContext(context.Background()).
		Raw(`SELECT status FROM aegis.account WHERE id = ?`, accountID).Scan(&status).Error)
	return status
}
