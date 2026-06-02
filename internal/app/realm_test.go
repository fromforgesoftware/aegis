package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestRealmCreate_Persists(t *testing.T) {
	repo := apptest.NewRealmRepository(t)
	uc := app.NewRealmUsecase(repo)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.Realm) bool {
		return r.Name() == "trading-bot"
	})).Return(domain.NewRealm("trading-bot", domain.WithRealmID("realm-1")), nil)

	got, err := uc.Create(context.Background(), domain.NewRealm("trading-bot",
		domain.WithRealmDisplayName("Trading Bot")))
	require.NoError(t, err)
	assert.Equal(t, "realm-1", got.ID())
}

func TestRealmCreate_RequiresName(t *testing.T) {
	repo := apptest.NewRealmRepository(t)
	uc := app.NewRealmUsecase(repo)
	_, err := uc.Create(context.Background(), domain.NewRealm(""))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
