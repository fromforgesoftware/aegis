//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestLoginSignalRepo verifies the risk-signal queries against real Postgres:
// seen-IP/device flip once recorded, and recent-failure counting respects the
// window and the succeeded flag.
func TestLoginSignalRepo(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "risk-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	acc, err := accRepo.Create(ctx, domain.NewAccount(realmID, "risk@x.com", "Risk User",
		domain.WithAccountType(domain.AccountTypeUser)))
	require.NoError(t, err)

	repo := db.NewLoginSignalRepository(client)

	// Unseen before any record.
	seen, err := repo.SeenIP(ctx, acc.ID(), "1.2.3.4")
	require.NoError(t, err)
	assert.False(t, seen)

	require.NoError(t, repo.Record(ctx, app.LoginSignal{
		AccountID: acc.ID(), RealmID: realmID, IP: "1.2.3.4", DeviceID: "dev-x", Succeeded: true,
	}))

	seen, err = repo.SeenIP(ctx, acc.ID(), "1.2.3.4")
	require.NoError(t, err)
	assert.True(t, seen, "IP is seen after a record")

	seenDev, err := repo.SeenDevice(ctx, acc.ID(), "dev-x")
	require.NoError(t, err)
	assert.True(t, seenDev)

	// Two recent failures + the earlier success → failures count is 2.
	for i := 0; i < 2; i++ {
		require.NoError(t, repo.Record(ctx, app.LoginSignal{
			AccountID: acc.ID(), RealmID: realmID, IP: "9.9.9.9", Succeeded: false,
		}))
	}
	failures, err := repo.RecentFailures(ctx, acc.ID(), time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 2, failures)

	// A window in the future excludes them.
	future, err := repo.RecentFailures(ctx, acc.ID(), time.Now().UTC().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, future)
}
