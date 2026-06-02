package app

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// PasswordResetTokenRepository persists single-use password-reset tokens —
// only the hash is stored.
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error
	// Consume atomically marks a valid token consumed and returns its
	// account id; NotFound if no valid token matches the hash.
	Consume(ctx context.Context, tokenHash string, now time.Time) (accountID string, err error)
}

// PasswordResetUsecase drives the forgotten-password flow: request a reset
// token (delivered via the NotificationSender) and confirm it with a new
// password.
type PasswordResetUsecase interface {
	RequestPasswordReset(ctx context.Context, realmID, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
}

const passwordResetTTL = 1 * time.Hour

type passwordResetUsecase struct {
	accounts AccountRepository
	tokens   PasswordResetTokenRepository
	creds    CredentialRepository
	hasher   PasswordHasher
	sender   NotificationSender
}

func NewPasswordResetUsecase(
	accounts AccountRepository,
	tokens PasswordResetTokenRepository,
	creds CredentialRepository,
	hasher PasswordHasher,
	sender NotificationSender,
) PasswordResetUsecase {
	return &passwordResetUsecase{accounts: accounts, tokens: tokens, creds: creds, hasher: hasher, sender: sender}
}

// RequestPasswordReset issues a reset token for the account and sends it.
// To avoid account enumeration it always succeeds: if no account matches,
// it silently does nothing (no token, no email) rather than erroring.
func (uc *passwordResetUsecase) RequestPasswordReset(ctx context.Context, realmID, email string) error {
	acc, err := uc.accounts.Get(ctx, byRealmEmail(realmID, normalizeEmail(email)))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil // don't reveal whether the email is registered
		}
		return err
	}
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return apierrors.InternalError("failed to generate reset token")
	}
	if err := uc.tokens.Create(ctx, acc.ID(), hash, time.Now().UTC().Add(passwordResetTTL)); err != nil {
		return err
	}
	return uc.sender.SendPasswordReset(ctx, acc.Email(), raw)
}

func (uc *passwordResetUsecase) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	// Reset enforces the default-policy floor here; realm-specific rules on
	// reset land with the flow API (Wave 2), where the realm is known up
	// front so a policy violation never burns the single-use token.
	if err := domain.ValidatePassword(domain.DefaultPasswordPolicy(), newPassword); err != nil {
		return apierrors.InvalidArgument(err.Error())
	}
	accountID, err := uc.tokens.Consume(ctx, hashToken(token), time.Now().UTC())
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return apierrors.InvalidArgument("invalid or expired reset token")
		}
		return err
	}
	cred, err := uc.hasher.Hash(newPassword)
	if err != nil {
		return apierrors.InternalError("failed to hash password")
	}
	return uc.creds.SetPassword(ctx, accountID, cred)
}
