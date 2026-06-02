package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeSessionState is the JSON:API type for /api/session-states.
const ResourceTypeSessionState resource.Type = "sessionStates"

// SessionState is the live topology of a session — where it currently is
// (realm/shard/region) and when it was last active. Its id is the session id.
type SessionState interface {
	resource.Resource
	AccountID() string
	CurrentRealmID() string
	CurrentShard() string
	Region() string
	IP() string
	UserAgent() string
	LastActive() time.Time
}

type sessionState struct {
	resource.Resource

	accountID      string
	currentRealmID string
	currentShard   string
	region         string
	ip             string
	userAgent      string
	lastActive     time.Time
}

type SessionStateOption func(*sessionState)

func WithSessionStateCurrentRealmID(id string) SessionStateOption {
	return func(s *sessionState) { s.currentRealmID = id }
}
func WithSessionStateCurrentShard(shard string) SessionStateOption {
	return func(s *sessionState) { s.currentShard = shard }
}
func WithSessionStateRegion(region string) SessionStateOption {
	return func(s *sessionState) { s.region = region }
}
func WithSessionStateIP(ip string) SessionStateOption {
	return func(s *sessionState) { s.ip = ip }
}
func WithSessionStateUserAgent(ua string) SessionStateOption {
	return func(s *sessionState) { s.userAgent = ua }
}
func WithSessionStateLastActive(t time.Time) SessionStateOption {
	return func(s *sessionState) { s.lastActive = t }
}

// NewSessionState builds session-state keyed by sessionID; region defaults to
// "default" for single-region deployments.
func NewSessionState(sessionID, accountID string, opts ...SessionStateOption) SessionState {
	s := &sessionState{
		Resource:  resource.New(resource.WithType(ResourceTypeSessionState), resource.WithID(sessionID)),
		accountID: accountID,
		region:    "default",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *sessionState) AccountID() string      { return s.accountID }
func (s *sessionState) CurrentRealmID() string { return s.currentRealmID }
func (s *sessionState) CurrentShard() string   { return s.currentShard }
func (s *sessionState) Region() string         { return s.region }
func (s *sessionState) IP() string             { return s.ip }
func (s *sessionState) UserAgent() string      { return s.userAgent }
func (s *sessionState) LastActive() time.Time  { return s.lastActive }
