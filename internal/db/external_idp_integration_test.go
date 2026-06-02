//go:build integration

package db_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestExternalIDPConfigRepository_RoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "idp-realm").Error)

	repo, err := db.NewExternalIDPConfigRepository(client)
	require.NoError(t, err)

	sealed := []byte{1, 2, 3, 4}
	created, err := repo.Create(ctx, domain.NewExternalIDPConfig(realmID, domain.ExternalIDPKindOAuthGoogle, "google-prod",
		domain.WithExternalIDPClientID("google-client"),
		domain.WithExternalIDPClientSecretEncrypted(sealed),
		domain.WithExternalIDPScopes([]string{"openid", "profile"}),
	))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())

	got, err := repo.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, created.ID())))
	require.NoError(t, err)
	assert.Equal(t, "google-prod", got.Name())
	assert.Equal(t, domain.ExternalIDPKindOAuthGoogle, got.Kind())
	assert.Equal(t, sealed, got.ClientSecretEncrypted(), "sealed bytes survive the BYTEA round-trip")
	assert.Equal(t, []string{"openid", "profile"}, got.Scopes())

	// Duplicate (realm, kind, name) is rejected.
	_, err = repo.Create(ctx, domain.NewExternalIDPConfig(realmID, domain.ExternalIDPKindOAuthGoogle, "google-prod"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists))

	// Listing filtered by realm returns the one row.
	list, err := repo.List(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.RealmID, realmID)))
	require.NoError(t, err)
	require.Len(t, list.Results(), 1)
}
