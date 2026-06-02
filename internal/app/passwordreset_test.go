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

func newPasswordReset(t *testing.T) (
	*apptest.AccountRepository,
	*apptest.PasswordResetTokenRepository,
	*apptest.CredentialRepository,
	*apptest.PasswordHasher,
	*apptest.NotificationSender,
	app.PasswordResetUsecase,
) {
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewPasswordResetTokenRepository(t)
	creds := apptest.NewCredentialRepository(t)
	hasher := apptest.NewPasswordHasher(t)
	sender := apptest.NewNotificationSender(t)
	uc := app.NewPasswordResetUsecase(accounts, tokens, creds, hasher, sender)
	return accounts, tokens, creds, hasher, sender, uc
}

func TestRequestPasswordReset_Success(t *testing.T) {
	accounts, tokens, _, _, sender, uc := newPasswordReset(t)
	acc := internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
		internaltest.WithAccountEmail("user@example.com"))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(acc, nil)
	tokens.EXPECT().Create(mock.Anything, "acc-1", mock.MatchedBy(isTokenHash), mock.Anything).Return(nil)
	sender.EXPECT().SendPasswordReset(mock.Anything, "user@example.com", mock.Anything).Return(nil)

	require.NoError(t, uc.RequestPasswordReset(context.Background(), "r", "user@example.com"))
}

func TestRequestPasswordReset_UnknownEmailSucceedsSilently(t *testing.T) {
	accounts, _, _, _, _, uc := newPasswordReset(t)
	// No token/sender calls; anti-enumeration means request returns nil.
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("account", ""))

	require.NoError(t, uc.RequestPasswordReset(context.Background(), "r", "nobody@example.com"))
}

func TestConfirmPasswordReset_Success(t *testing.T) {
	_, tokens, creds, hasher, _, uc := newPasswordReset(t)
	// Confirm must hash the raw token before consuming it.
	tokens.EXPECT().Consume(mock.Anything, mock.MatchedBy(isTokenHash), mock.Anything).Return("acc-1", nil)
	hasher.EXPECT().Hash("newpassword123").Return(app.HashedPassword{Encoded: "enc", Algo: "argon2id"}, nil)
	creds.EXPECT().SetPassword(mock.Anything, "acc-1", mock.Anything).Return(nil)

	require.NoError(t, uc.ConfirmPasswordReset(context.Background(), "raw-token", "newpassword123"))
}

func TestConfirmPasswordReset_InvalidToken(t *testing.T) {
	_, tokens, _, _, _, uc := newPasswordReset(t)
	tokens.EXPECT().Consume(mock.Anything, mock.MatchedBy(isTokenHash), mock.Anything).
		Return("", apierrors.NotFound("password reset token", ""))

	err := uc.ConfirmPasswordReset(context.Background(), "bad", "newpassword123")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)
}

func TestConfirmPasswordReset_ShortPassword(t *testing.T) {
	_, _, _, _, _, uc := newPasswordReset(t)
	// Rejected before the token is consumed (no Consume call).
	err := uc.ConfirmPasswordReset(context.Background(), "raw-token", "short")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
