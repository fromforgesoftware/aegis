package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
)

func TestSweep_RefreshesWhenGrantsRemoved(t *testing.T) {
	bindings := apptest.NewBindingRepository(t)
	authz := apptest.NewAuthorizationUsecase(t)
	bindings.EXPECT().DeleteExpired(mock.Anything, mock.Anything).Return(int64(2), nil)
	authz.EXPECT().Refresh(mock.Anything).Return(nil)

	removed, err := app.NewGrantSweeper(bindings, authz).Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)
}

func TestSweep_SkipsRefreshWhenNothingExpiredAndProjectionCurrent(t *testing.T) {
	bindings := apptest.NewBindingRepository(t)
	authz := apptest.NewAuthorizationUsecase(t)
	// No Refresh expectation: nothing expired and the projection has already
	// published every write, so the sweep must be a no-op.
	bindings.EXPECT().DeleteExpired(mock.Anything, mock.Anything).Return(int64(0), nil)
	authz.EXPECT().Version(mock.Anything).Return(int64(7), int64(7), nil)

	removed, err := app.NewGrantSweeper(bindings, authz).Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), removed)
}

func TestSweep_RefreshesWhenProjectionBehind(t *testing.T) {
	bindings := apptest.NewBindingRepository(t)
	authz := apptest.NewAuthorizationUsecase(t)
	// Nothing expired, but an earlier write was never published (its writer's
	// explicit Refresh failed) — the sweep must republish it.
	bindings.EXPECT().DeleteExpired(mock.Anything, mock.Anything).Return(int64(0), nil)
	authz.EXPECT().Version(mock.Anything).Return(int64(9), int64(7), nil)
	authz.EXPECT().Refresh(mock.Anything).Return(nil)

	removed, err := app.NewGrantSweeper(bindings, authz).Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), removed)
}
