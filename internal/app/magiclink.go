package app

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// MagicLinkTokenRepository persists single-use passwordless-login tokens —
// only the hash is stored, the raw token lives in the email.
type MagicLinkTokenRepository interface {
	Create(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error
	// Consume atomically marks a valid token consumed and returns its account
	// id; NotFound if no valid token matches the hash.
	Consume(ctx context.Context, tokenHash string, now time.Time) (accountID string, err error)
}

// MagicLinkUsecase drives passwordless email login: request a one-time link
// (delivered via the NotificationSender) and redeem it to authenticate.
type MagicLinkUsecase interface {
	RequestMagicLink(ctx context.Context, realmID, email string) error
	// RedeemMagicLink consumes a valid token and returns the authenticated
	// account; the caller mints the session/tokens. A banned or disabled
	// account is rejected.
	RedeemMagicLink(ctx context.Context, token string) (domain.Account, error)
}

const magicLinkTTL = 15 * time.Minute

type magicLinkUsecase struct {
	accounts AccountRepository
	tokens   MagicLinkTokenRepository
	sender   NotificationSender
}

func NewMagicLinkUsecase(
	accounts AccountRepository,
	tokens MagicLinkTokenRepository,
	sender NotificationSender,
) MagicLinkUsecase {
	return &magicLinkUsecase{accounts: accounts, tokens: tokens, sender: sender}
}

// RequestMagicLink issues a login token for the account and emails it. Like
// password reset it always succeeds even when no account matches, so the
// response never reveals whether an email is registered.
func (uc *magicLinkUsecase) RequestMagicLink(ctx context.Context, realmID, email string) error {
	acc, err := uc.accounts.Get(ctx, byRealmEmail(realmID, normalizeEmail(email)))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil // don't reveal whether the email is registered
		}
		return err
	}
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return apierrors.InternalError("failed to generate magic-link token")
	}
	if err := uc.tokens.Create(ctx, acc.ID(), hash, time.Now().UTC().Add(magicLinkTTL)); err != nil {
		return err
	}
	return uc.sender.SendMagicLink(ctx, acc.Email(), raw)
}

func (uc *magicLinkUsecase) RedeemMagicLink(ctx context.Context, token string) (domain.Account, error) {
	if token == "" {
		return nil, apierrors.InvalidArgument("token is required")
	}
	accountID, err := uc.tokens.Consume(ctx, hashToken(token), time.Now().UTC())
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil, apierrors.Unauthenticated("invalid or expired magic link")
		}
		return nil, err
	}
	acc, err := uc.accounts.Get(ctx, byID(accountID))
	if err != nil {
		return nil, err
	}
	if acc.Status() == domain.AccountStatusBanned || acc.Status() == domain.AccountStatusDisabled {
		return nil, apierrors.Unauthenticated("account is not active")
	}
	return acc, nil
}
