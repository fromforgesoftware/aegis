//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
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

func byID(id string) search.Option {
	return search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, id))
}

// TestFlowRepository_CRUD exercises the flow repo against real Postgres:
// create a pending flow, read it back, complete it via Patch, and confirm
// an unknown id is a clean NotFound.
func TestFlowRepository_CRUD(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()

	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "flow-realm").Error)

	repo, err := db.NewFlowRepository(client)
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewFlow(realmID, domain.FlowTypeLogin, time.Now().Add(30*time.Minute).UTC()))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())
	assert.Equal(t, domain.FlowStatePending, created.State())
	assert.Equal(t, domain.FlowTypeLogin, created.FlowType())

	got, err := repo.Get(ctx, byID(created.ID()))
	require.NoError(t, err)
	assert.Equal(t, realmID, got.RealmID())
	assert.Equal(t, domain.FlowStatePending, got.State())

	updated, err := repo.Patch(ctx,
		repository.PatchSearchOpts(byID(created.ID())),
		repository.WithPatchFields(map[string]any{fields.State: string(domain.FlowStateCompleted)}),
	)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Equal(t, domain.FlowStateCompleted, updated[0].State())

	_, err = repo.Get(ctx, byID(uuid.NewString()))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND, got %v", err)
}
