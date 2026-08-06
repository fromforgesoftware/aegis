//go:build integration

package db_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// seedOrg inserts one organization (with its required anchor resource) into realmID.
func seedOrg(t *testing.T, ctx context.Context, realmID, name, slug string) string {
	t.Helper()
	client := internaltest.GetDB(t)

	resourceID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.resource (id, realm_id, type) VALUES (?, ?, ?)`,
		resourceID, realmID, "organizations").Error)

	orgID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.organization (id, realm_id, resource_id, name, slug) VALUES (?, ?, ?, ?, ?)`,
		orgID, realmID, resourceID, name, slug).Error)
	return orgID
}

// TestOrganizationPatch_DuplicateSlugIsAConflict pins the mapping of the UNIQUE (realm_id, slug)
// violation onto 409.
//
// It is an integration test because the behaviour under test belongs to Postgres: the constraint
// is what rejects the write, and a unit test with a mocked DB could only assert that aegis
// mishandles an error it invented. Before this mapping existed, renaming a workspace onto a taken
// slug surfaced as a 500 — indistinguishable from a service outage, for the one failure the person
// filling in the form could actually fix.
func TestOrganizationPatch_DuplicateSlugIsAConflict(t *testing.T) {
	ctx := context.Background()
	client := internaltest.GetDB(t)

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "org-patch-realm").Error)

	seedOrg(t, ctx, realmID, "Taken", "taken")
	mover := seedOrg(t, ctx, realmID, "Mover", "mover")

	repo, err := db.NewOrganizationRepository(client)
	require.NoError(t, err)

	_, err = repo.Patch(ctx,
		repository.PatchSearchOpts(search.WithQueryOpts(
			query.FilterBy(filter.OpEq, fields.ID, mover))),
		repository.PatchField(fields.Slug, "taken"),
	)
	require.Error(t, err, "renaming onto an existing slug must fail")

	apiErr, ok := apierrors.As(err)
	require.True(t, ok, "want a kit error carrying an HTTP status, got %#v", err)
	require.Equal(t, http.StatusConflict, apiErr.HTTPStatus(),
		"a taken slug is a conflict the caller can resolve, not a server error")

	// A slug that is free still applies, so the mapping above has not turned every patch into a
	// conflict.
	patched, err := repo.Patch(ctx,
		repository.PatchSearchOpts(search.WithQueryOpts(
			query.FilterBy(filter.OpEq, fields.ID, mover))),
		repository.PatchField(fields.Slug, "moved"),
	)
	require.NoError(t, err)
	require.Len(t, patched, 1)
	require.Equal(t, "moved", patched[0].Slug())
}
