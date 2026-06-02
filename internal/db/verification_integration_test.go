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

// captureSender stands in for the NotificationSender so the test can read
// the raw token that would have been emailed.
type captureSender struct{ to, token string }

func (c *captureSender) SendEmailVerification(_ context.Context, to, token string) error {
	c.to, c.token = to, token
	return nil
}

func (c *captureSender) SendPasswordReset(_ context.Context, to, token string) error {
	c.to, c.token = to, token
	return nil
}

func (c *captureSender) SendInvitation(_ context.Context, to, token string) error {
	c.to, c.token = to, token
	return nil
}

func (c *captureSender) SendMagicLink(_ context.Context, to, token string) error {
	c.to, c.token = to, token
	return nil
}

// TestEmailVerification exercises the full flow against real Postgres:
// request issues a hashed token + delivers the raw one, verify consumes it
// and flips email_verified, and the token is single-use.
func TestEmailVerification(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "verify-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	tokenRepo, err := db.NewVerificationTokenRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	tx := gormdb.NewTransactioner(client, logger.New())

	authx := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(), tx)
	sender := &captureSender{}
	verify := app.NewVerificationUsecase(accRepo, tokenRepo, sender)

	const email = "verify@example.com"
	acc, err := authx.Register(ctx, app.RegisterInput{
		RealmID: realmID, Email: email, Password: "correct horse battery staple", DisplayName: "V",
	})
	require.NoError(t, err)

	// Request → a token is generated, hashed + stored, and the raw token sent.
	require.NoError(t, verify.RequestEmailVerification(ctx, acc.ID()))
	require.Equal(t, email, sender.to)
	require.NotEmpty(t, sender.token)

	// Verify with the raw token → email_verified flips on the profile row.
	require.NoError(t, verify.VerifyEmail(ctx, sender.token))

	var verified bool
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT email_verified FROM aegis.user_account WHERE account_id = ?`, acc.ID()).
		Row().Scan(&verified))
	assert.True(t, verified, "email should be verified")

	// Single-use: replaying the same token is rejected.
	err = verify.VerifyEmail(ctx, sender.token)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
