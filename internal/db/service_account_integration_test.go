//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestServiceAccountRepo verifies the service_account repo against real
// Postgres: a row is created against a SERVICE account, resolved by client_id
// with its secret hash, listed, last-used stamped, and deleted.
func TestServiceAccountRepo(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "svc-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	// A SERVICE account is the authz subject the service_account row points at.
	acc, err := accRepo.Create(ctx, domain.NewAccount(realmID, "client-1@svc.local", "ci-bot",
		domain.WithAccountType(domain.AccountTypeService)))
	require.NoError(t, err)

	repo, err := db.NewServiceAccountRepository(client)
	require.NoError(t, err)

	sa := domain.NewServiceAccount(realmID, "ci-bot", "client-1",
		domain.WithServiceAccountID(acc.ID()), domain.WithServiceAccountScopes([]string{"audit:read"}))
	created, err := repo.Create(ctx, sa, "secret-hash")
	require.NoError(t, err)
	assert.Equal(t, acc.ID(), created.ID())

	got, hash, err := repo.GetByClientID(ctx, realmID, "client-1")
	require.NoError(t, err)
	assert.Equal(t, "secret-hash", hash)
	assert.Equal(t, []string{"audit:read"}, got.Scopes())

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list.Results(), 1)

	require.NoError(t, repo.TouchLastUsed(ctx, acc.ID(), time.Now().UTC()))
	stamped, _, err := repo.GetByClientID(ctx, realmID, "client-1")
	require.NoError(t, err)
	assert.NotNil(t, stamped.LastUsedAt())

	require.NoError(t, repo.Delete(ctx, acc.ID()))
	_, _, err = repo.GetByClientID(ctx, realmID, "client-1")
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}
