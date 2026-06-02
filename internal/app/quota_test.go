package app_test

import (
	"context"
	"errors"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestQuotaAllow_UnderCap(t *testing.T) {
	policies := apptest.NewQuotaPolicyRepository(t)
	uc := app.NewQuotaUsecase(policies)
	policies.EXPECT().GetByRealmResourceType(mock.Anything, "r", "character").
		Return(internaltest.NewQuotaPolicy(internaltest.WithQuotaPolicyMaxCount(3)), nil)

	allowed, err := uc.Allow(context.Background(), "r", "character", 2)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestQuotaAllow_AtCapRejected(t *testing.T) {
	policies := apptest.NewQuotaPolicyRepository(t)
	uc := app.NewQuotaUsecase(policies)
	policies.EXPECT().GetByRealmResourceType(mock.Anything, "r", "character").
		Return(internaltest.NewQuotaPolicy(internaltest.WithQuotaPolicyMaxCount(3)), nil)

	allowed, err := uc.Allow(context.Background(), "r", "character", 3)
	require.NoError(t, err)
	assert.False(t, allowed, "at the cap, no room left")
}

func TestQuotaAllow_NoPolicyUnconstrained(t *testing.T) {
	policies := apptest.NewQuotaPolicyRepository(t)
	uc := app.NewQuotaUsecase(policies)
	policies.EXPECT().GetByRealmResourceType(mock.Anything, "r", "character").
		Return(nil, apierrors.NotFound("realm quota policy", ""))

	allowed, err := uc.Allow(context.Background(), "r", "character", 9999)
	require.NoError(t, err)
	assert.True(t, allowed, "a realm with no policy is unconstrained")
}

func TestQuotaCheck_UsesRegisteredCounter(t *testing.T) {
	policies := apptest.NewQuotaPolicyRepository(t)
	uc := app.NewQuotaUsecase(policies)
	counter := apptest.NewUsageCounter(t)
	counter.EXPECT().Count(mock.Anything, "r").Return(5, nil)
	uc.Register("character", counter)
	policies.EXPECT().GetByRealmResourceType(mock.Anything, "r", "character").
		Return(internaltest.NewQuotaPolicy(internaltest.WithQuotaPolicyMaxCount(5)), nil)

	allowed, err := uc.Check(context.Background(), "r", "character")
	require.NoError(t, err)
	assert.False(t, allowed, "counter reports 5, cap is 5 → full")
}

func TestQuotaCheck_NoCounterRegistered(t *testing.T) {
	policies := apptest.NewQuotaPolicyRepository(t)
	uc := app.NewQuotaUsecase(policies)
	_, err := uc.Check(context.Background(), "r", "unknown")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestQuotaCheck_CounterError(t *testing.T) {
	policies := apptest.NewQuotaPolicyRepository(t)
	uc := app.NewQuotaUsecase(policies)
	counter := apptest.NewUsageCounter(t)
	counter.EXPECT().Count(mock.Anything, "r").Return(0, errors.New("boom"))
	uc.Register("character", counter)

	_, err := uc.Check(context.Background(), "r", "character")
	require.Error(t, err)
}
