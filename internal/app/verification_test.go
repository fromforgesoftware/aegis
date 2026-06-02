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
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// isTokenHash asserts a value is a SHA-256 hex digest (64 chars) — i.e. the
// usecase stored/consumed the *hash* of the token, never the raw value.
// Used inside mock.MatchedBy for token-repo arg assertions.
func isTokenHash(h string) bool { return len(h) == 64 }

func TestRequestEmailVerification_Success(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewVerificationTokenRepository(t)
	sender := apptest.NewNotificationSender(t)

	acc := internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
		internaltest.WithAccountEmail("user@example.com"))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(acc, nil)
	tokens.EXPECT().Create(mock.Anything, "acc-1", mock.MatchedBy(isTokenHash), mock.Anything).Return(nil)
	sender.EXPECT().SendEmailVerification(mock.Anything, "user@example.com", mock.Anything).Return(nil)

	uc := app.NewVerificationUsecase(accounts, tokens, sender)
	require.NoError(t, uc.RequestEmailVerification(context.Background(), "acc-1"))
}

func TestRequestEmailVerification_AccountNotFound(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("account", "acc-1"))

	// No token/sender calls when the account doesn't exist.
	uc := app.NewVerificationUsecase(accounts, apptest.NewVerificationTokenRepository(t), apptest.NewNotificationSender(t))
	err := uc.RequestEmailVerification(context.Background(), "acc-1")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}

func TestVerifyEmail_Success(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewVerificationTokenRepository(t)

	// VerifyEmail must hash the raw token before consuming it.
	tokens.EXPECT().Consume(mock.Anything, mock.MatchedBy(isTokenHash), mock.Anything).Return("acc-1", nil)
	accounts.EXPECT().MarkEmailVerified(mock.Anything, "acc-1").Return(nil)

	uc := app.NewVerificationUsecase(accounts, tokens, apptest.NewNotificationSender(t))
	require.NoError(t, uc.VerifyEmail(context.Background(), "raw-token"))
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	tokens := apptest.NewVerificationTokenRepository(t)
	tokens.EXPECT().Consume(mock.Anything, mock.MatchedBy(isTokenHash), mock.Anything).
		Return("", apierrors.NotFound("verification token", ""))

	// No MarkEmailVerified call when the token is invalid.
	uc := app.NewVerificationUsecase(apptest.NewAccountRepository(t), tokens, apptest.NewNotificationSender(t))
	err := uc.VerifyEmail(context.Background(), "bad")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)
}
