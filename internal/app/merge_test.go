package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newMergeUsecase(t *testing.T) (*apptest.AccountMergeRepository, *apptest.AccountRepository, *apptest.AuthorizationUsecase, app.AccountMergeUsecase) {
	merge := apptest.NewAccountMergeRepository(t)
	accounts := apptest.NewAccountRepository(t)
	authz := apptest.NewAuthorizationUsecase(t)
	uc := app.NewAccountMergeUsecase(merge, accounts, authz, persistencetest.NewTransactioner(), app.NoopAuditor{})
	return merge, accounts, authz, uc
}

func TestMerge_TransfersAndRefreshes(t *testing.T) {
	merge, accounts, authz, uc := newMergeUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("src"), internaltest.WithAccountRealmID("r")), nil).Once()
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("dst"), internaltest.WithAccountRealmID("r")), nil).Once()

	merge.EXPECT().TransferExternalIDs(mock.Anything, "src", "dst").Return(int64(2), nil)
	merge.EXPECT().TransferMemberships(mock.Anything, "src", "dst").Return(int64(1), nil)
	merge.EXPECT().TransferBindings(mock.Anything, "src", "dst").Return(int64(3), nil)
	merge.EXPECT().SoftDeleteSource(mock.Anything, "src").Return(nil)
	merge.EXPECT().RecordMergeEvent(mock.Anything, mock.MatchedBy(func(e app.AccountMergeEvent) bool {
		return e.SourceID == "src" && e.TargetID == "dst" && e.RealmID == "r" && e.Summary.Bindings == 3
	})).Return(nil)
	// Bindings moved → the projection must be refreshed so they enter the closure.
	authz.EXPECT().Refresh(mock.Anything).Return(nil)

	got, err := uc.Merge(context.Background(), "src", "dst")
	require.NoError(t, err)
	assert.Equal(t, int64(3), got.Bindings)
	assert.Equal(t, int64(2), got.ExternalIDs)
}

func TestMerge_NoBindingsSkipsRefresh(t *testing.T) {
	merge, accounts, authz, uc := newMergeUsecase(t)
	_ = authz // Refresh must NOT be called; mockery fails the test if it is.

	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("src"), internaltest.WithAccountRealmID("r")), nil).Once()
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("dst"), internaltest.WithAccountRealmID("r")), nil).Once()
	merge.EXPECT().TransferExternalIDs(mock.Anything, "src", "dst").Return(int64(0), nil)
	merge.EXPECT().TransferMemberships(mock.Anything, "src", "dst").Return(int64(0), nil)
	merge.EXPECT().TransferBindings(mock.Anything, "src", "dst").Return(int64(0), nil)
	merge.EXPECT().SoftDeleteSource(mock.Anything, "src").Return(nil)
	merge.EXPECT().RecordMergeEvent(mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Merge(context.Background(), "src", "dst")
	require.NoError(t, err)
}

func TestMerge_RejectsSelfMerge(t *testing.T) {
	_, _, _, uc := newMergeUsecase(t)
	_, err := uc.Merge(context.Background(), "same", "same")
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestMerge_RejectsCrossRealm(t *testing.T) {
	_, accounts, _, uc := newMergeUsecase(t)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("src"), internaltest.WithAccountRealmID("r1")), nil).Once()
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("dst"), internaltest.WithAccountRealmID("r2")), nil).Once()

	_, err := uc.Merge(context.Background(), "src", "dst")
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
