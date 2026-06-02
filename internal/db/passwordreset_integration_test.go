//go:build integration

package db_test

import (
	"context"
	"testing"

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

// TestPasswordReset exercises the full forgotten-password flow against real
// Postgres: request issues a token, confirm sets a new password, the new
// password logs in (old one no longer does), and the token is single-use.
func TestPasswordReset(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "reset-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	resetTokens, err := db.NewPasswordResetTokenRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	hasher := app.NewArgon2idHasher()
	tx := gormdb.NewTransactioner(client, logger.New())

	authx := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, hasher, tx)
	sender := &captureSender{}
	reset := app.NewPasswordResetUsecase(accRepo, resetTokens, credRepo, hasher, sender)

	const (
		email = "reset@example.com"
		oldPw = "the old password value"
		newPw = "a brand new password value"
	)
	_, err = authx.Register(ctx, app.RegisterInput{RealmID: realmID, Email: email, Password: oldPw, DisplayName: "R"})
	require.NoError(t, err)

	// Request → a reset token is issued and the raw token captured.
	require.NoError(t, reset.RequestPasswordReset(ctx, realmID, email))
	require.NotEmpty(t, sender.token)

	// Confirm with the new password.
	require.NoError(t, reset.ConfirmPasswordReset(ctx, sender.token, newPw))

	// The new password logs in; the old one no longer does.
	_, err = authx.Login(ctx, app.LoginInput{RealmID: realmID, Email: email, Password: newPw})
	require.NoError(t, err)

	_, err = authx.Login(ctx, app.LoginInput{RealmID: realmID, Email: email, Password: oldPw})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "old password should fail")

	// Single-use: replaying the reset token is rejected.
	err = reset.ConfirmPasswordReset(ctx, sender.token, "yet another password")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
