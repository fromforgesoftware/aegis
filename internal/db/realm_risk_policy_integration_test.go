//go:build integration

package db_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestRealmRiskPolicyUpsert verifies the per-realm risk policy against real
// Postgres: absent → NotFound, upsert stores the weights, and a second upsert
// replaces them in place (one row per realm).
func TestRealmRiskPolicyUpsert(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "risk-policy-realm").Error)

	repo, err := db.NewRealmRiskPolicyRepository(client)
	require.NoError(t, err)

	_, err = repo.GetByRealm(ctx, realmID)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))

	_, err = repo.Upsert(ctx, domain.NewRealmRiskPolicy(realmID, domain.RiskPolicy{
		NewIPWeight: 30, NewDeviceWeight: 40, FailureWeight: 15, StepUpThreshold: 40, DenyThreshold: 90,
	}))
	require.NoError(t, err)

	got, err := repo.GetByRealm(ctx, realmID)
	require.NoError(t, err)
	assert.Equal(t, 40, got.Policy().StepUpThreshold)

	// Replace in place.
	_, err = repo.Upsert(ctx, domain.NewRealmRiskPolicy(realmID, domain.RiskPolicy{
		NewIPWeight: 10, NewDeviceWeight: 10, FailureWeight: 5, StepUpThreshold: 25, DenyThreshold: 60,
	}))
	require.NoError(t, err)
	got, err = repo.GetByRealm(ctx, realmID)
	require.NoError(t, err)
	assert.Equal(t, 25, got.Policy().StepUpThreshold)
	assert.Equal(t, 60, got.Policy().DenyThreshold)

	var count int64
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM aegis.realm_risk_policy WHERE realm_id = ?`, realmID).Scan(&count).Error)
	assert.Equal(t, int64(1), count, "upsert keeps one row per realm")
}
