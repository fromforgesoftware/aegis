//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestMagicLinkTokenConsume verifies single-use semantics against a real
// Postgres: a valid token is consumed exactly once, a second redeem fails,
// and an expired token never matches.
func TestMagicLinkTokenConsume(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "magic-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	authx := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(),
		gormdb.NewTransactioner(client, logger.New()))
	acc, err := authx.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "ml@x.com", Password: "correct horse battery staple"})
	require.NoError(t, err)

	tokens, err := db.NewMagicLinkTokenRepository(client)
	require.NoError(t, err)
	now := time.Now().UTC()

	require.NoError(t, tokens.Create(ctx, acc.ID(), "hash-valid", now.Add(15*time.Minute)))
	require.NoError(t, tokens.Create(ctx, acc.ID(), "hash-expired", now.Add(-time.Minute)))

	// First consume of the valid token returns the account.
	got, err := tokens.Consume(ctx, "hash-valid", now)
	require.NoError(t, err)
	assert.Equal(t, acc.ID(), got)

	// Second consume fails — single-use.
	_, err = tokens.Consume(ctx, "hash-valid", now)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))

	// Expired token never matches.
	_, err = tokens.Consume(ctx, "hash-expired", now)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}
