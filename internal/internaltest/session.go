package internaltest

import (
	"time"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

type SessionOption func(*sessionOpts)

type sessionOpts struct {
	id        string
	realmID   string
	accountID string
	expiresAt time.Time
}

func defaultSessionOptions() []SessionOption {
	return []SessionOption{
		WithSessionID("session-test"),
		WithSessionRealmID("realm-test"),
		WithSessionAccountID("account-test"),
		WithSessionExpiresAt(time.Now().Add(time.Hour)),
	}
}

func WithSessionID(id string) SessionOption {
	return func(o *sessionOpts) { o.id = id }
}
func WithSessionRealmID(realmID string) SessionOption {
	return func(o *sessionOpts) { o.realmID = realmID }
}
func WithSessionAccountID(accountID string) SessionOption {
	return func(o *sessionOpts) { o.accountID = accountID }
}
func WithSessionExpiresAt(t time.Time) SessionOption {
	return func(o *sessionOpts) { o.expiresAt = t }
}

// NewSession builds a domain.Session fixture from defaults overridden by opts.
func NewSession(opts ...SessionOption) domain.Session {
	o := &sessionOpts{}
	for _, opt := range append(defaultSessionOptions(), opts...) {
		opt(o)
	}
	return domain.NewSession(o.realmID, o.accountID, o.expiresAt, domain.WithSessionID(o.id))
}

// MatchSession compares the identifying fields (realm + account), ignoring
// server-volatile id/timestamps. Use inside mock.MatchedBy.
func MatchSession(want domain.Session) func(domain.Session) bool {
	return func(got domain.Session) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() && want.AccountID() == got.AccountID()
	}
}
