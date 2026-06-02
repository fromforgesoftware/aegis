// Package app holds Aegis's use cases: the orchestration layer that
// composes domain logic over the repository and service ports. Transport
// adapters call into these; the ports are implemented by internal/db.
package app

import (
	"context"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// AccountRepository persists the Account aggregate (aegis.account +
// aegis.user_account) via the kit's generic Creator/Getter/Patcher, so
// reads and updates go through the search/query DSL.
type AccountRepository interface {
	repository.Creator[domain.Account]
	repository.Getter[domain.Account]
	repository.Patcher[domain.Account]
	// MarkEmailVerified flips user_account.email_verified. It's an explicit
	// profile-table transition because the generic Patcher targets the
	// account root, not the profile table where email_verified lives.
	MarkEmailVerified(ctx context.Context, accountID string) error
	// Ban/Unban/RestoreExpiredBans manage the moderation lifecycle on the
	// account root (status + banned_until + ban_reason).
	Ban(ctx context.Context, accountID string, until *time.Time, reason string) error
	Unban(ctx context.Context, accountID string) error
	RestoreExpiredBans(ctx context.Context, now time.Time) (int64, error)
}

// CredentialRepository persists password credentials, kept separate from
// the Account aggregate because the secret is write-only and has its own
// lifecycle (rotation, reset).
type CredentialRepository interface {
	// SetPassword upserts the password credential for an account.
	SetPassword(ctx context.Context, accountID string, cred HashedPassword) error
	// GetPasswordHash returns the PHC-encoded credential, or NotFound.
	GetPasswordHash(ctx context.Context, accountID string) (string, error)
}

// RegisterInput is the native password-registration command.
type RegisterInput struct {
	RealmID     string
	Email       string
	Password    string
	DisplayName string
}

// LoginInput is the native password-login command.
type LoginInput struct {
	RealmID  string
	Email    string
	Password string
	// IP and DeviceID feed risk-based auth when a RiskUsecase is wired; both
	// are optional (risk is skipped when IP is empty).
	IP       string
	DeviceID string
}

// AuthxUsecase is Aegis's unified auth surface. Wave 2: native password
// register + login. Token issuance lands in Wave 3; authorization folds
// in from Wave 5.
type AuthxUsecase interface {
	Register(ctx context.Context, in RegisterInput) (domain.Account, error)
	Login(ctx context.Context, in LoginInput) (domain.Account, error)
}

type authxUsecase struct {
	accounts AccountRepository
	creds    CredentialRepository
	policies PasswordPolicyRepository
	hasher   PasswordHasher
	tx       persistence.Transactioner
	lockout  domain.LockoutPolicy
	risk     RiskUsecase
}

// AuthxOption configures the authx usecase.
type AuthxOption func(*authxUsecase)

// WithRisk wires risk-based auth: a successful credential check is assessed and
// a DENY decision blocks the login. Optional — without it, login is unchanged.
func WithRisk(r RiskUsecase) AuthxOption {
	return func(uc *authxUsecase) { uc.risk = r }
}

func NewAuthxUsecase(
	accounts AccountRepository,
	creds CredentialRepository,
	policies PasswordPolicyRepository,
	hasher PasswordHasher,
	tx persistence.Transactioner,
	opts ...AuthxOption,
) AuthxUsecase {
	uc := &authxUsecase{
		accounts: accounts,
		creds:    creds,
		policies: policies,
		hasher:   hasher,
		tx:       tx,
		lockout:  domain.DefaultLockoutPolicy(),
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// byRealmEmail builds the search filter for an account in a realm by
// (normalized) email — the canonical login/uniqueness lookup.
func byRealmEmail(realmID, email string) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Email, email),
	)
}

func (uc *authxUsecase) Register(ctx context.Context, in RegisterInput) (domain.Account, error) {
	email := normalizeEmail(in.Email)
	if in.RealmID == "" {
		return nil, apierrors.InvalidArgument("realm_id is required")
	}
	if email == "" {
		return nil, apierrors.InvalidArgument("email is required")
	}
	if err := validatePassword(ctx, uc.policies, in.RealmID, in.Password); err != nil {
		return nil, err
	}

	// Per-realm email uniqueness (the realm key lives on the account row,
	// so it can't be a cross-table DB constraint — enforced here).
	if _, err := uc.accounts.Get(ctx, byRealmEmail(in.RealmID, email)); err == nil {
		return nil, apierrors.AlreadyExists("account", email)
	} else if !apierrors.Is(err, apierrors.CodeNotFound) {
		return nil, err
	}

	cred, err := uc.hasher.Hash(in.Password)
	if err != nil {
		return nil, apierrors.InternalError("failed to hash password")
	}

	acc := domain.NewAccount(in.RealmID, email, in.DisplayName,
		domain.WithAccountType(domain.AccountTypeUser),
		domain.WithAccountStatus(domain.AccountStatusEnabled),
	)

	// Account row + profile + credential commit atomically. The kit
	// transactioner threads the tx through ctx so both repos enlist in it.
	var out domain.Account
	err = uc.tx.Exec(ctx, func(ctx context.Context) error {
		created, err := uc.accounts.Create(ctx, acc)
		if err != nil {
			return err
		}
		if err := uc.creds.SetPassword(ctx, created.ID(), cred); err != nil {
			return err
		}
		out = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (uc *authxUsecase) Login(ctx context.Context, in LoginInput) (domain.Account, error) {
	email := normalizeEmail(in.Email)

	// Uniform "invalid credentials" for unknown-account and wrong-password
	// so we don't leak which emails are registered.
	acc, err := uc.accounts.Get(ctx, byRealmEmail(in.RealmID, email))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil, apierrors.Unauthenticated("invalid credentials")
		}
		return nil, err
	}
	if acc.Status() != domain.AccountStatusEnabled {
		return nil, apierrors.Unauthenticated("invalid credentials")
	}

	now := time.Now().UTC()
	if uc.lockout.IsLocked(acc.LockedUntil(), now) {
		return nil, apierrors.RateLimited("account temporarily locked due to too many failed login attempts")
	}

	encoded, err := uc.creds.GetPasswordHash(ctx, acc.ID())
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil, apierrors.Unauthenticated("invalid credentials")
		}
		return nil, err
	}
	ok, err := uc.hasher.Verify(in.Password, encoded)
	if err != nil || !ok {
		uc.recordRisk(ctx, acc, in, false)
		return nil, uc.recordFailedLogin(ctx, acc, now)
	}

	// Risk gate: a DENY decision blocks an otherwise-valid login (e.g. a new
	// device amid a burst of recent failures). STEP_UP elevation into the
	// token layer is a follow-up; here DENY is the enforced action.
	if uc.riskDenies(ctx, acc, in) {
		return nil, apierrors.Forbidden("login blocked by risk policy")
	}

	// Success: stamp last login and clear the lockout counters.
	if _, err := uc.accounts.Patch(ctx,
		repository.PatchSearchOpts(byID(acc.ID())),
		repository.WithPatchFields(map[string]any{
			fields.LastLoginAt:      now,
			fields.FailedLoginCount: 0,
			fields.LockedUntil:      nil,
		}),
	); err != nil {
		return nil, err
	}
	return acc, nil
}

// recordFailedLogin persists the incremented failure count (and a lock
// expiry once the policy threshold is crossed), then returns the uniform
// invalid-credentials error.
func (uc *authxUsecase) recordFailedLogin(ctx context.Context, acc domain.Account, now time.Time) error {
	count, lockedUntil := uc.lockout.OnFailure(acc.FailedLoginCount(), now)
	if _, err := uc.accounts.Patch(ctx,
		repository.PatchSearchOpts(byID(acc.ID())),
		repository.WithPatchFields(map[string]any{
			fields.FailedLoginCount: count,
			fields.LockedUntil:      lockedUntil,
		}),
	); err != nil {
		return err
	}
	return apierrors.Unauthenticated("invalid credentials")
}

// riskDenies assesses a successful login and reports whether the risk policy
// blocks it. It fails open: when risk is unwired, the IP is absent, or the
// assessment errors, the login proceeds (availability over a hard block).
func (uc *authxUsecase) riskDenies(ctx context.Context, acc domain.Account, in LoginInput) bool {
	a, ok := uc.assess(ctx, acc, in, true)
	return ok && a.Decision == domain.RiskDeny
}

// recordRisk records a failed attempt as a risk signal so future assessments
// see the failure history. Best-effort.
func (uc *authxUsecase) recordRisk(ctx context.Context, acc domain.Account, in LoginInput, succeeded bool) {
	uc.assess(ctx, acc, in, succeeded)
}

func (uc *authxUsecase) assess(ctx context.Context, acc domain.Account, in LoginInput, succeeded bool) (domain.RiskAssessment, bool) {
	if uc.risk == nil || in.IP == "" {
		return domain.RiskAssessment{}, false
	}
	a, err := uc.risk.Assess(ctx, RiskInput{
		RealmID: acc.RealmID(), AccountID: acc.ID(), IP: in.IP, DeviceID: in.DeviceID, Succeeded: succeeded,
	})
	if err != nil {
		return domain.RiskAssessment{}, false
	}
	return a, true
}

// byID filters by the account's primary key.
func byID(id string) search.Option {
	return search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, id))
}

// byRealm filters by realm id — the lookup for per-realm singletons such
// as the password policy.
func byRealm(realmID string) search.Option {
	return search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.RealmID, realmID))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
