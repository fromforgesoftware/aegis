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

// TestRegisterThenLogin exercises the full Wave-2 password path against a
// real Postgres: register writes account + profile + credential in one
// transaction; login verifies the argon2id hash; duplicates and wrong
// passwords are rejected.
func TestRegisterThenLogin(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	// A realm is required (account.realm_id FK).
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "test-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	tx := gormdb.NewTransactioner(client, logger.New())
	uc := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(), tx)

	const email = "Trader@Example.com"
	const password = "correct horse battery staple"

	// Register.
	acc, err := uc.Register(ctx, app.RegisterInput{
		RealmID:     realmID,
		Email:       email,
		Password:    password,
		DisplayName: "Trader",
	})
	require.NoError(t, err)
	require.NotEmpty(t, acc.ID())
	assert.Equal(t, "trader@example.com", acc.Email(), "email is normalized lower-case")

	// Duplicate registration (same realm + email) is rejected.
	_, err = uc.Register(ctx, app.RegisterInput{RealmID: realmID, Email: email, Password: password})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists), "want ALREADY_EXISTS, got %v", err)

	// Login with the correct password returns the same account.
	got, err := uc.Login(ctx, app.LoginInput{RealmID: realmID, Email: email, Password: password})
	require.NoError(t, err)
	assert.Equal(t, acc.ID(), got.ID())

	// Login stamps last_login_at on the account root via the generic Patch.
	var stamped int64
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM aegis.account WHERE id = ? AND last_login_at IS NOT NULL`, acc.ID()).
		Scan(&stamped).Error)
	assert.Equal(t, int64(1), stamped, "login should stamp last_login_at")

	// Wrong password is rejected as UNAUTHENTICATED.
	_, err = uc.Login(ctx, app.LoginInput{RealmID: realmID, Email: email, Password: "wrong"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "want UNAUTHENTICATED, got %v", err)

	// Unknown email is also UNAUTHENTICATED (no account enumeration).
	_, err = uc.Login(ctx, app.LoginInput{RealmID: realmID, Email: "nobody@example.com", Password: password})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

// TestLoginLockout verifies brute-force protection end-to-end: after the
// policy's threshold of consecutive failures the account is locked, and a
// subsequent login is rejected as rate-limited even with the right password.
func TestLoginLockout(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "lockout-realm").Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	uc := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(),
		gormdb.NewTransactioner(client, logger.New()))

	const email = "lock@example.com"
	const password = "correct horse battery staple"

	acc, err := uc.Register(ctx, app.RegisterInput{RealmID: realmID, Email: email, Password: password, DisplayName: "Lock"})
	require.NoError(t, err)

	// Default policy locks after 5 consecutive failures; each wrong attempt
	// (incl. the 5th, which sets the lock) is reported as invalid credentials.
	for i := 1; i <= 5; i++ {
		_, err := uc.Login(ctx, app.LoginInput{RealmID: realmID, Email: email, Password: "wrong"})
		require.Error(t, err)
		require.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "attempt %d: want UNAUTHENTICATED, got %v", i, err)
	}

	// Once locked, even the correct password is rejected as rate-limited.
	_, err = uc.Login(ctx, app.LoginInput{RealmID: realmID, Email: email, Password: password})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeRateLimited), "want RATE_LIMITED, got %v", err)

	// The lockout state is persisted on the account row.
	var count int
	var locked bool
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT failed_login_count, locked_until IS NOT NULL FROM aegis.account WHERE id = ?`, acc.ID()).
		Row().Scan(&count, &locked))
	assert.Equal(t, 5, count)
	assert.True(t, locked, "locked_until should be set after the threshold")
}
