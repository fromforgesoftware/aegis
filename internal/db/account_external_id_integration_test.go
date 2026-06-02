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

func TestAccountExternalIDRepository_RoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "link-realm").Error)
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)

	repo, err := db.NewAccountExternalIDRepository(client)
	require.NoError(t, err)

	link := domain.AccountExternalID{
		AccountID: accountID, Kind: domain.ExternalIDPKindOAuthGoogle, ExternalID: "google-uid-1",
	}
	require.NoError(t, repo.Create(ctx, link))

	got, err := repo.GetByExternalID(ctx, domain.ExternalIDPKindOAuthGoogle, "google-uid-1")
	require.NoError(t, err)
	assert.Equal(t, accountID, got.AccountID)

	// (kind, external_id) is globally unique: a second account can't claim it.
	other := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, other, realmID).Error)
	err = repo.Create(ctx, domain.AccountExternalID{
		AccountID: other, Kind: domain.ExternalIDPKindOAuthGoogle, ExternalID: "google-uid-1",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists))

	// ListByAccount enumerates the links one account holds.
	links, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, domain.ExternalIDPKindOAuthGoogle, links[0].Kind)

	_, err = repo.GetByExternalID(ctx, domain.ExternalIDPKindOAuthGitHub, "ghost")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}
