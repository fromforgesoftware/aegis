//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/audit"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestAuditEventSink_AppendsAndIsImmutable(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	sink := db.NewAuditEventSink(client)

	err := sink.Emit(ctx, audit.Event{
		Action:       "binding.grant",
		ResourceType: "binding",
		ResourceID:   uuid.NewString(),
		ActorID:      "acc-1",
		ActorType:    "ACCOUNT",
		Changes:      map[string]any{"roleId": "role-1"},
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT count(*) FROM aegis.audit_event WHERE action = 'binding.grant'`).Scan(&count).Error)
	assert.Equal(t, int64(1), count)

	// JSONB changes round-trip via the GIN-indexed column.
	var role string
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT changes->>'roleId' FROM aegis.audit_event LIMIT 1`).Scan(&role).Error)
	assert.Equal(t, "role-1", role)

	// Append-only: UPDATE and DELETE are rejected by the trigger.
	err = client.WithContext(ctx).Exec(`UPDATE aegis.audit_event SET action = 'tampered'`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "append-only")
	err = client.WithContext(ctx).Exec(`DELETE FROM aegis.audit_event`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "append-only")
}
