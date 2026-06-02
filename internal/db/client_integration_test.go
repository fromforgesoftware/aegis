//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// TestClientRepository_CRUD exercises the OIDC client repo against real
// Postgres: create (with JSONB arrays), read by id, list by realm, the
// per-realm client_id uniqueness, and soft delete.
func TestClientRepository_CRUD(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "client-realm").Error)

	repo, err := db.NewClientRepository(client)
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewClient(realmID, "web", domain.ClientTypeConfidential, "Web App",
		domain.WithClientSecretHash("hash"),
		domain.WithClientGrantTypes([]string{"authorization_code", "refresh_token"}),
		domain.WithClientScopes([]string{"openid", "profile"}),
		domain.WithClientRedirectURIs([]string{"https://app.example.com/callback"}),
	))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())

	got, err := repo.Get(ctx, byID(created.ID()))
	require.NoError(t, err)
	assert.Equal(t, "web", got.ClientID())
	assert.Equal(t, domain.ClientTypeConfidential, got.ClientType())
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, got.GrantTypes())
	assert.Equal(t, []string{"https://app.example.com/callback"}, got.RedirectURIs())

	list, err := repo.List(ctx, byRealm(realmID))
	require.NoError(t, err)
	require.Len(t, list.Results(), 1)

	// Per-realm client_id uniqueness.
	_, err = repo.Create(ctx, domain.NewClient(realmID, "web", domain.ClientTypePublic, "Dup"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists), "want ALREADY_EXISTS, got %v", err)

	require.NoError(t, repo.Delete(ctx, repository.DeleteTypeSoft, byID(created.ID())))
	_, err = repo.Get(ctx, byID(created.ID()))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND after delete, got %v", err)
}
