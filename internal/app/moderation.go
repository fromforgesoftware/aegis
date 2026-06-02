package app

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

// AccountModerationUsecase bans and unbans accounts. Bans live on the account
// root (status + banned_until + ban_reason); a nil until is permanent.
type AccountModerationUsecase interface {
	Ban(ctx context.Context, accountID string, until *time.Time, reason string) error
	Unban(ctx context.Context, accountID string) error
}

type accountModerationUsecase struct {
	accounts AccountRepository
	auditor  Auditor
}

func NewAccountModerationUsecase(accounts AccountRepository, auditor Auditor) AccountModerationUsecase {
	return &accountModerationUsecase{accounts: accounts, auditor: auditor}
}

func (uc *accountModerationUsecase) Ban(ctx context.Context, accountID string, until *time.Time, reason string) error {
	if accountID == "" {
		return apierrors.InvalidArgument("account_id is required")
	}
	if until != nil && !until.After(time.Now()) {
		return apierrors.InvalidArgument("until must be in the future")
	}
	if _, err := uc.accounts.Get(ctx, byID(accountID)); err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return apierrors.NotFound("account", accountID)
		}
		return err
	}
	if err := uc.accounts.Ban(ctx, accountID, until, reason); err != nil {
		return err
	}
	uc.auditor.Record(ctx, "account.ban", "account", accountID, map[string]any{
		"until":  until,
		"reason": reason,
	})
	return nil
}

func (uc *accountModerationUsecase) Unban(ctx context.Context, accountID string) error {
	if accountID == "" {
		return apierrors.InvalidArgument("account_id is required")
	}
	if err := uc.accounts.Unban(ctx, accountID); err != nil {
		return err
	}
	uc.auditor.Record(ctx, "account.unban", "account", accountID, nil)
	return nil
}

// BanSweeper restores temporary bans whose expiry has passed. It runs on an
// interval ticker so a timed ban lifts itself without operator action.
type BanSweeper interface {
	Sweep(ctx context.Context) (int64, error)
}

type banSweeper struct {
	accounts AccountRepository
	now      func() time.Time
}

func NewBanSweeper(accounts AccountRepository) BanSweeper {
	return &banSweeper{accounts: accounts, now: time.Now}
}

func (s *banSweeper) Sweep(ctx context.Context) (int64, error) {
	return s.accounts.RestoreExpiredBans(ctx, s.now())
}
