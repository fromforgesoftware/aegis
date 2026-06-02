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

	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestSigningKeyRepository_RoundTrip stores an envelope-sealed key in real
// Postgres via the generic Create, reads it back via the generic Get/List
// (filtered by realm + status), and confirms the sealed BYTEA opens to the
// original key.
func TestSigningKeyRepository_RoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "key-realm").Error)

	repo, err := db.NewSigningKeyRepository(client)
	require.NoError(t, err)
	cipher, err := cryptox.NewCipher(make([]byte, 32))
	require.NoError(t, err)

	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	der, err := cryptox.MarshalPrivateKey(key)
	require.NoError(t, err)
	sealed, err := cipher.Seal(der)
	require.NoError(t, err)
	jwk, err := cryptox.PublicJWK(&key.PublicKey, "kid-1")
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewSigningKey(realmID, "kid-1", "RS256", jwk, sealed))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())
	assert.Equal(t, domain.SigningKeyStatusActive, created.Status())

	got, err := repo.Get(ctx, byRealmStatus(realmID, domain.SigningKeyStatusActive))
	require.NoError(t, err)
	assert.Equal(t, "kid-1", got.Kid())

	// The sealed private key survived the BYTEA round-trip and still opens.
	openedDER, err := cipher.Open(got.SealedPrivateKey())
	require.NoError(t, err)
	openedKey, err := cryptox.ParsePrivateKey(openedDER)
	require.NoError(t, err)
	assert.Equal(t, key.N, openedKey.N)

	list, err := repo.List(ctx, byRealmStatus(realmID, domain.SigningKeyStatusActive))
	require.NoError(t, err)
	require.Len(t, list.Results(), 1)

	_, err = repo.Get(ctx, byRealmStatus(uuid.NewString(), domain.SigningKeyStatusActive))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND, got %v", err)
}

func byRealmStatus(realmID string, status domain.SigningKeyStatus) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Status, string(status)),
	)
}
