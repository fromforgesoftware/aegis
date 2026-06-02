// Package domain holds Aegis's pure business types — no I/O, no kit
// transport/persistence imports beyond the shared resource model.
package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeAccount is the JSON:API type for /api/accounts.
const ResourceTypeAccount resource.Type = "accounts"

// AccountType discriminates human users from machine identities.
type AccountType string

const (
	AccountTypeUser    AccountType = "USER"
	AccountTypeService AccountType = "SERVICE"
)

func (t AccountType) Valid() bool {
	switch t {
	case AccountTypeUser, AccountTypeService:
		return true
	}
	return false
}

// AccountStatus is the lifecycle marker for an account.
type AccountStatus string

const (
	AccountStatusCreated  AccountStatus = "CREATED"
	AccountStatusEnabled  AccountStatus = "ENABLED"
	AccountStatusDisabled AccountStatus = "DISABLED"
	AccountStatusBanned   AccountStatus = "BANNED"
)

func (s AccountStatus) Valid() bool {
	switch s {
	case AccountStatusCreated, AccountStatusEnabled, AccountStatusDisabled, AccountStatusBanned:
		return true
	}
	return false
}

// Account is Aegis's identity aggregate: one aegis.account row plus its
// 1:1 aegis.user_account profile, loaded together. AccountType() reports
// USER/SERVICE; the embedded resource.Resource.Type() reports the
// JSON:API type ("accounts"), so the two never clash.
type Account interface {
	resource.Resource
	RealmID() string
	AccountType() AccountType
	Status() AccountStatus
	Email() string
	EmailVerified() bool
	DisplayName() string
	PhotoURL() string
	LastLoginAt() *time.Time
	FailedLoginCount() int
	LockedUntil() *time.Time
}

type account struct {
	resource.Resource

	realmID          string
	accountType      AccountType
	status           AccountStatus
	email            string
	emailVerified    bool
	displayName      string
	photoURL         string
	lastLoginAt      *time.Time
	failedLoginCount int
	lockedUntil      *time.Time
}

type AccountOption func(*account)

func WithAccountID(id string) AccountOption {
	return func(a *account) { a.Resource = resource.Update(a.Resource, resource.WithID(id)) }
}

func WithAccountType(t AccountType) AccountOption {
	return func(a *account) { a.accountType = t }
}

func WithAccountStatus(s AccountStatus) AccountOption {
	return func(a *account) { a.status = s }
}

func WithAccountEmailVerified(v bool) AccountOption {
	return func(a *account) { a.emailVerified = v }
}

func WithAccountPhotoURL(p string) AccountOption {
	return func(a *account) { a.photoURL = p }
}

func WithAccountLastLoginAt(t *time.Time) AccountOption {
	return func(a *account) { a.lastLoginAt = t }
}

func WithAccountFailedLoginCount(n int) AccountOption {
	return func(a *account) { a.failedLoginCount = n }
}

func WithAccountLockedUntil(t *time.Time) AccountOption {
	return func(a *account) { a.lockedUntil = t }
}

// NewAccount builds an Account aggregate. Defaults: type USER, status
// ENABLED. Override via options.
func NewAccount(realmID, email, displayName string, opts ...AccountOption) Account {
	a := &account{
		Resource:    resource.New(resource.WithType(ResourceTypeAccount)),
		realmID:     realmID,
		accountType: AccountTypeUser,
		status:      AccountStatusEnabled,
		email:       email,
		displayName: displayName,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *account) RealmID() string          { return a.realmID }
func (a *account) AccountType() AccountType { return a.accountType }
func (a *account) Status() AccountStatus    { return a.status }
func (a *account) Email() string            { return a.email }
func (a *account) EmailVerified() bool      { return a.emailVerified }
func (a *account) DisplayName() string      { return a.displayName }
func (a *account) PhotoURL() string         { return a.photoURL }
func (a *account) LastLoginAt() *time.Time  { return a.lastLoginAt }
func (a *account) FailedLoginCount() int    { return a.failedLoginCount }
func (a *account) LockedUntil() *time.Time  { return a.lockedUntil }
