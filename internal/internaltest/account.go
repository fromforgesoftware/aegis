package internaltest

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// account is a test stub implementing domain.Account. Used by mock
// matchers and fixtures so unit tests don't depend on the GORM-backed
// entity in db/ or on the domain constructor's defaults.
type account struct {
	id               string
	createdAt        time.Time
	updatedAt        time.Time
	deletedAt        *time.Time
	realmID          string
	accountType      domain.AccountType
	status           domain.AccountStatus
	email            string
	emailVerified    bool
	displayName      string
	photoURL         string
	lastLoginAt      *time.Time
	failedLoginCount int
	lockedUntil      *time.Time
}

type AccountOption func(*account)

func defaultAccountOptions() []AccountOption {
	return []AccountOption{
		WithAccountID("acc-test"),
		WithAccountRealmID("realm-test"),
		WithAccountType(domain.AccountTypeUser),
		WithAccountStatus(domain.AccountStatusEnabled),
		WithAccountEmail("test@example.com"),
		WithAccountDisplayName("Test User"),
	}
}

func WithAccountID(id string) AccountOption {
	return func(a *account) { a.id = id }
}
func WithAccountRealmID(realmID string) AccountOption {
	return func(a *account) { a.realmID = realmID }
}
func WithAccountType(t domain.AccountType) AccountOption {
	return func(a *account) { a.accountType = t }
}
func WithAccountStatus(s domain.AccountStatus) AccountOption {
	return func(a *account) { a.status = s }
}
func WithAccountEmail(e string) AccountOption {
	return func(a *account) { a.email = e }
}
func WithAccountEmailVerified(v bool) AccountOption {
	return func(a *account) { a.emailVerified = v }
}
func WithAccountDisplayName(n string) AccountOption {
	return func(a *account) { a.displayName = n }
}
func WithAccountLockedUntil(t *time.Time) AccountOption {
	return func(a *account) { a.lockedUntil = t }
}
func WithAccountFailedLoginCount(n int) AccountOption {
	return func(a *account) { a.failedLoginCount = n }
}

// NewAccount builds a domain.Account fixture from the defaults overridden
// by opts.
func NewAccount(opts ...AccountOption) domain.Account {
	a := &account{}
	for _, opt := range append(defaultAccountOptions(), opts...) {
		opt(a)
	}
	return a
}

func (a *account) ID() string                      { return a.id }
func (a *account) LID() string                     { return "" }
func (a *account) Type() resource.Type             { return domain.ResourceTypeAccount }
func (a *account) CreatedAt() time.Time            { return a.createdAt }
func (a *account) UpdatedAt() time.Time            { return a.updatedAt }
func (a *account) DeletedAt() *time.Time           { return a.deletedAt }
func (a *account) RealmID() string                 { return a.realmID }
func (a *account) AccountType() domain.AccountType { return a.accountType }
func (a *account) Status() domain.AccountStatus    { return a.status }
func (a *account) Email() string                   { return a.email }
func (a *account) EmailVerified() bool             { return a.emailVerified }
func (a *account) DisplayName() string             { return a.displayName }
func (a *account) PhotoURL() string                { return a.photoURL }
func (a *account) LastLoginAt() *time.Time         { return a.lastLoginAt }
func (a *account) FailedLoginCount() int           { return a.failedLoginCount }
func (a *account) LockedUntil() *time.Time         { return a.lockedUntil }

// MatchAccount compares the identifying profile fields (realm, email,
// type, status, display name), ignoring server-volatile id/timestamps.
// Use it inside mock.MatchedBy for repo Create/Update arg assertions.
func MatchAccount(want domain.Account) func(domain.Account) bool {
	return func(got domain.Account) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.Email() == got.Email() &&
			want.AccountType() == got.AccountType() &&
			want.Status() == got.Status() &&
			want.DisplayName() == got.DisplayName()
	}
}

// MatchAccountWithID is MatchAccount plus an id check, for when the test
// pins the identity (e.g. a load returned the expected row).
func MatchAccountWithID(want domain.Account) func(domain.Account) bool {
	base := MatchAccount(want)
	return func(got domain.Account) bool {
		return base(got) && want.ID() == got.ID()
	}
}
