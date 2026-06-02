package internaltest

import (
	"time"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

type SessionStateOption func(*sessionStateOpts)

type sessionStateOpts struct {
	sessionID      string
	accountID      string
	currentRealmID string
	currentShard   string
	region         string
	ip             string
	userAgent      string
	lastActive     time.Time
}

func defaultSessionStateOptions() []SessionStateOption {
	return []SessionStateOption{
		WithSessionStateSessionID("session-test"),
		WithSessionStateAccountID("account-test"),
	}
}

func WithSessionStateSessionID(id string) SessionStateOption {
	return func(o *sessionStateOpts) { o.sessionID = id }
}
func WithSessionStateAccountID(id string) SessionStateOption {
	return func(o *sessionStateOpts) { o.accountID = id }
}
func WithSessionStateCurrentRealmID(id string) SessionStateOption {
	return func(o *sessionStateOpts) { o.currentRealmID = id }
}
func WithSessionStateCurrentShard(shard string) SessionStateOption {
	return func(o *sessionStateOpts) { o.currentShard = shard }
}
func WithSessionStateRegion(region string) SessionStateOption {
	return func(o *sessionStateOpts) { o.region = region }
}
func WithSessionStateLastActive(t time.Time) SessionStateOption {
	return func(o *sessionStateOpts) { o.lastActive = t }
}

func NewSessionState(opts ...SessionStateOption) domain.SessionState {
	o := &sessionStateOpts{}
	for _, opt := range append(defaultSessionStateOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.SessionStateOption{
		domain.WithSessionStateCurrentRealmID(o.currentRealmID),
		domain.WithSessionStateCurrentShard(o.currentShard),
		domain.WithSessionStateIP(o.ip),
		domain.WithSessionStateUserAgent(o.userAgent),
	}
	if o.region != "" {
		domainOpts = append(domainOpts, domain.WithSessionStateRegion(o.region))
	}
	if !o.lastActive.IsZero() {
		domainOpts = append(domainOpts, domain.WithSessionStateLastActive(o.lastActive))
	}
	return domain.NewSessionState(o.sessionID, o.accountID, domainOpts...)
}

// MatchSessionState compares id + account + current shard, ignoring timestamps.
func MatchSessionState(want domain.SessionState) func(domain.SessionState) bool {
	return func(got domain.SessionState) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.ID() == got.ID() &&
			want.AccountID() == got.AccountID() &&
			want.CurrentShard() == got.CurrentShard()
	}
}
