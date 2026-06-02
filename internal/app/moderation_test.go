package app_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestModerationBan_BansAndAudits(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	auditor := &recordingAuditor{}
	uc := app.NewAccountModerationUsecase(accounts, auditor)

	until := time.Now().Add(24 * time.Hour)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1")), nil)
	accounts.EXPECT().Ban(mock.Anything, "acct-1", &until, "spam").Return(nil)

	require.NoError(t, uc.Ban(context.Background(), "acct-1", &until, "spam"))
	assert.Equal(t, "account.ban", auditor.action)
	assert.Equal(t, "acct-1", auditor.resourceID)
}

func TestModerationBan_RejectsEmptyAccount(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	uc := app.NewAccountModerationUsecase(accounts, &recordingAuditor{})

	err := uc.Ban(context.Background(), "", nil, "")
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestModerationBan_RejectsPastExpiry(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	uc := app.NewAccountModerationUsecase(accounts, &recordingAuditor{})

	past := time.Now().Add(-time.Hour)
	err := uc.Ban(context.Background(), "acct-1", &past, "")
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestModerationBan_NotFound(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	uc := app.NewAccountModerationUsecase(accounts, &recordingAuditor{})

	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("account", "acct-x"))

	err := uc.Ban(context.Background(), "acct-x", nil, "")
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}

func TestModerationUnban_UnbansAndAudits(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	auditor := &recordingAuditor{}
	uc := app.NewAccountModerationUsecase(accounts, auditor)

	accounts.EXPECT().Unban(mock.Anything, "acct-1").Return(nil)

	require.NoError(t, uc.Unban(context.Background(), "acct-1"))
	assert.Equal(t, "account.unban", auditor.action)
}

func TestBanSweeper_RestoresExpired(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	sweeper := app.NewBanSweeper(accounts)

	accounts.EXPECT().RestoreExpiredBans(mock.Anything, mock.MatchedBy(func(ts time.Time) bool {
		return !ts.IsZero()
	})).Return(int64(3), nil)

	n, err := sweeper.Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}
