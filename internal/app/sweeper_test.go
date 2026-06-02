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

func TestSweep_SkipsRefreshWhenNothingExpired(t *testing.T) {
	bindings := apptest.NewBindingRepository(t)
	authz := apptest.NewAuthorizationUsecase(t)
	// No Refresh expectation: a no-op sweep must not refresh the projection.
	bindings.EXPECT().DeleteExpired(mock.Anything, mock.Anything).Return(int64(0), nil)

	removed, err := app.NewGrantSweeper(bindings, authz).Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), removed)
}
