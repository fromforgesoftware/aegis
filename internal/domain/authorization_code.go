package domain

import "time"

// AuthorizationCode is the short-lived, single-use grant artifact bridging /authorize and /token.
type AuthorizationCode struct {
	Code          string
	RealmID       string
	ClientID      string
	AccountID     string
	RedirectURI   string
	SessionID     string
	Scopes        []string
	PKCEChallenge string
	Nonce         string
	ExpiresAt     time.Time
}
