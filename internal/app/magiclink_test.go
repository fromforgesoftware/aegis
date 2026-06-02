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
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestRequestMagicLink_IssuesAndSends(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewMagicLinkTokenRepository(t)
	sender := apptest.NewNotificationSender(t)
	uc := app.NewMagicLinkUsecase(accounts, tokens, sender)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"), internaltest.WithAccountEmail("user@example.com")), nil)
	tokens.EXPECT().Create(mock.Anything, "acc-1", mock.Anything, mock.Anything).Return(nil)
	sender.EXPECT().SendMagicLink(mock.Anything, "user@example.com", mock.Anything).Return(nil)

	require.NoError(t, uc.RequestMagicLink(context.Background(), "realm-1", "User@Example.com"))
}

func TestRequestMagicLink_UnknownEmailSucceedsSilently(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewMagicLinkTokenRepository(t)
	sender := apptest.NewNotificationSender(t)
	uc := app.NewMagicLinkUsecase(accounts, tokens, sender)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("account", "x"))

	// No token created, no email sent — and no error (no enumeration).
	require.NoError(t, uc.RequestMagicLink(context.Background(), "realm-1", "nobody@example.com"))
}

func TestRedeemMagicLink_ReturnsAccount(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewMagicLinkTokenRepository(t)
	uc := app.NewMagicLinkUsecase(accounts, tokens, apptest.NewNotificationSender(t))

	tokens.EXPECT().Consume(mock.Anything, mock.Anything, mock.Anything).Return("acc-1", nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
			internaltest.WithAccountStatus(domain.AccountStatusEnabled)), nil)

	acc, err := uc.RedeemMagicLink(context.Background(), "raw-token")
	require.NoError(t, err)
	assert.Equal(t, "acc-1", acc.ID())
}

func TestRedeemMagicLink_InvalidToken(t *testing.T) {
	tokens := apptest.NewMagicLinkTokenRepository(t)
	uc := app.NewMagicLinkUsecase(apptest.NewAccountRepository(t), tokens, apptest.NewNotificationSender(t))

	tokens.EXPECT().Consume(mock.Anything, mock.Anything, mock.Anything).
		Return("", apierrors.NotFound("magic link token", ""))

	_, err := uc.RedeemMagicLink(context.Background(), "bad")
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestRedeemMagicLink_BannedAccountRejected(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewMagicLinkTokenRepository(t)
	uc := app.NewMagicLinkUsecase(accounts, tokens, apptest.NewNotificationSender(t))

	tokens.EXPECT().Consume(mock.Anything, mock.Anything, mock.Anything).Return("acc-1", nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
			internaltest.WithAccountStatus(domain.AccountStatusBanned)), nil)

	_, err := uc.RedeemMagicLink(context.Background(), "raw-token")
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}
