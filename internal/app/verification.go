package app

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

// VerificationTokenRepository persists single-use email-verification
// tokens — only the hash is stored.
type VerificationTokenRepository interface {
	Create(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error
	// Consume atomically marks a valid token consumed and returns its
	// account id; NotFound if no valid token matches the hash.
	Consume(ctx context.Context, tokenHash string, now time.Time) (accountID string, err error)
}

// VerificationUsecase drives the email-verification flow: issue a token
// (delivered via the NotificationSender) and verify it.
type VerificationUsecase interface {
	RequestEmailVerification(ctx context.Context, accountID string) error
	VerifyEmail(ctx context.Context, token string) error
}

const emailVerificationTTL = 24 * time.Hour

type verificationUsecase struct {
	accounts AccountRepository
	tokens   VerificationTokenRepository
	sender   NotificationSender
}

func NewVerificationUsecase(
	accounts AccountRepository,
	tokens VerificationTokenRepository,
	sender NotificationSender,
) VerificationUsecase {
	return &verificationUsecase{accounts: accounts, tokens: tokens, sender: sender}
}

func (uc *verificationUsecase) RequestEmailVerification(ctx context.Context, accountID string) error {
	acc, err := uc.accounts.Get(ctx, byID(accountID))
	if err != nil {
		return err
	}
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return apierrors.InternalError("failed to generate verification token")
	}
	if err := uc.tokens.Create(ctx, accountID, hash, time.Now().UTC().Add(emailVerificationTTL)); err != nil {
		return err
	}
	return uc.sender.SendEmailVerification(ctx, acc.Email(), raw)
}

func (uc *verificationUsecase) VerifyEmail(ctx context.Context, token string) error {
	accountID, err := uc.tokens.Consume(ctx, hashToken(token), time.Now().UTC())
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return apierrors.InvalidArgument("invalid or expired verification token")
		}
		return err
	}
	return uc.accounts.MarkEmailVerified(ctx, accountID)
}
