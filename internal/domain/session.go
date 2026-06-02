package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeSession is the JSON:API type for sessions (admin inspect/revoke).
const ResourceTypeSession resource.Type = "sessions"

// Session is a browser login session: the hosted-login cookie carries its
// id, and /authorize resolves the authenticated account from it.
type Session interface {
	resource.Resource
	RealmID() string
	AccountID() string
	ExpiresAt() time.Time
	RevokedAt() *time.Time
}

type session struct {
	resource.Resource

	realmID   string
	accountID string
	expiresAt time.Time
	revokedAt *time.Time
}

type SessionOption func(*session)

func WithSessionID(id string) SessionOption {
	return func(s *session) { s.Resource = resource.Update(s.Resource, resource.WithID(id)) }
}

func NewSession(realmID, accountID string, expiresAt time.Time, opts ...SessionOption) Session {
	s := &session{
		Resource:  resource.New(resource.WithType(ResourceTypeSession)),
		realmID:   realmID,
		accountID: accountID,
		expiresAt: expiresAt,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *session) RealmID() string       { return s.realmID }
func (s *session) AccountID() string     { return s.accountID }
func (s *session) ExpiresAt() time.Time  { return s.expiresAt }
func (s *session) RevokedAt() *time.Time { return s.revokedAt }

// SessionActive reports whether s can still authenticate a request at now.
func SessionActive(s Session, now time.Time) bool {
	return s.RevokedAt() == nil && now.Before(s.ExpiresAt())
}
