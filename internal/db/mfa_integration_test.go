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

func TestMFARepositories_RoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "mfa-realm").Error)
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)

	enrollments, err := db.NewMFAEnrollmentRepository(client)
	require.NoError(t, err)
	recovery, err := db.NewRecoveryCodeRepository(client)
	require.NoError(t, err)
	stepup, err := db.NewStepUpTokenRepository(client)
	require.NoError(t, err)

	// Enrollment upsert is idempotent on (account, factor) and confirmable.
	_, err = enrollments.Upsert(ctx, domain.NewMFAEnrollment(accountID, domain.MFAFactorTOTP,
		domain.WithMFAEnrollmentSecret("sealed-v1")))
	require.NoError(t, err)
	_, err = enrollments.Upsert(ctx, domain.NewMFAEnrollment(accountID, domain.MFAFactorTOTP,
		domain.WithMFAEnrollmentSecret("sealed-v2")))
	require.NoError(t, err)
	got, err := enrollments.GetByAccountFactor(ctx, accountID, domain.MFAFactorTOTP)
	require.NoError(t, err)
	assert.Equal(t, "sealed-v2", got.Secret())
	require.NoError(t, enrollments.Confirm(ctx, accountID, domain.MFAFactorTOTP, time.Now()))

	// Recovery codes are one-time: a second consume of the same code fails.
	require.NoError(t, recovery.CreateMany(ctx, accountID, []string{"hash-a", "hash-b"}))
	first, err := recovery.Consume(ctx, accountID, "hash-a", time.Now())
	require.NoError(t, err)
	assert.True(t, first)
	again, err := recovery.Consume(ctx, accountID, "hash-a", time.Now())
	require.NoError(t, err)
	assert.False(t, again, "a used recovery code can't be consumed twice")

	// Step-up token verifies while live and rejects once expired.
	live := domain.NewStepUpToken("tok-live", accountID, domain.MFAFactorTOTP, "aal2", time.Now().Add(time.Hour))
	require.NoError(t, stepup.Create(ctx, live))
	_, err = stepup.Verify(ctx, "tok-live", time.Now())
	require.NoError(t, err)

	expired := domain.NewStepUpToken("tok-old", accountID, domain.MFAFactorTOTP, "aal2", time.Now().Add(-time.Hour))
	require.NoError(t, stepup.Create(ctx, expired))
	_, err = stepup.Verify(ctx, "tok-old", time.Now())
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodePreconditionFailed))
}

func TestRealmACRPolicyRepository_Upsert(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "acr-realm").Error)

	repo, err := db.NewRealmACRPolicyRepository(client)
	require.NoError(t, err)

	_, err = repo.Upsert(ctx, domain.NewRealmACRPolicy(realmID, true, "aal2"))
	require.NoError(t, err)
	// Upsert again (same realm) flips the flag without a duplicate row.
	_, err = repo.Upsert(ctx, domain.NewRealmACRPolicy(realmID, false, "aal3"))
	require.NoError(t, err)

	got, err := repo.GetByRealm(ctx, realmID)
	require.NoError(t, err)
	assert.False(t, got.MFARequired())
	assert.Equal(t, "aal3", got.RequiredACR())
}
