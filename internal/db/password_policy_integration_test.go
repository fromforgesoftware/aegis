//go:build integration

package db_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func byRealm(realmID string) search.Option {
	return search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.RealmID, realmID))
}

// TestPasswordPolicyRepository_Get checks both paths: a realm with a
// configured policy row returns those rules; a realm with none is a clean
// NotFound (the usecase, not the repo, applies the default fallback).
func TestPasswordPolicyRepository_Get(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "policy-realm").Error)
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.password_policy (realm_id, min_length, require_digit, require_symbol)
		 VALUES (?, ?, ?, ?)`, realmID, 12, true, true).Error)

	repo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)

	// Configured realm → the stored rules.
	got, err := repo.Get(ctx, byRealm(realmID))
	require.NoError(t, err)
	assert.Equal(t, 12, got.MinLength())
	assert.True(t, got.RequireDigit())
	assert.True(t, got.RequireSymbol())
	assert.False(t, got.RequireUppercase())

	// Realm with no row → NotFound.
	_, err = repo.Get(ctx, byRealm(uuid.NewString()))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND, got %v", err)
}

// TestRegister_EnforcesRealmPasswordPolicy verifies the policy is applied at
// registration: a realm with a strict policy rejects a weak password and
// accepts a compliant one.
func TestRegister_EnforcesRealmPasswordPolicy(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "strict-realm").Error)
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.password_policy
		   (realm_id, min_length, require_uppercase, require_lowercase, require_digit, require_symbol)
		 VALUES (?, ?, ?, ?, ?, ?)`, realmID, 12, true, true, true, true).Error)

	accRepo, err := db.NewAccountRepository(client)
	require.NoError(t, err)
	credRepo, err := db.NewCredentialRepository(client)
	require.NoError(t, err)
	policyRepo, err := db.NewPasswordPolicyRepository(client)
	require.NoError(t, err)
	uc := app.NewAuthxUsecase(accRepo, credRepo, policyRepo, app.NewArgon2idHasher(),
		gormdb.NewTransactioner(client, logger.New()))

	// 12 chars but lowercase-only → violates the realm policy.
	_, err = uc.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "weak@x.com", Password: "alllowercase", DisplayName: "W"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)

	// Satisfies every rule → registers.
	acc, err := uc.Register(ctx, app.RegisterInput{RealmID: realmID, Email: "strong@x.com", Password: "Str0ng!Passphrase", DisplayName: "S"})
	require.NoError(t, err)
	require.NotEmpty(t, acc.ID())
}
