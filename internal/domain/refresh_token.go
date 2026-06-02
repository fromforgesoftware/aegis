package domain

import "time"

// RefreshToken is an opaque, rotated OAuth refresh token: hashed at rest, single-use, with rotated_from chaining for reuse detection.
type RefreshToken struct {
	ID          string
	SessionID   string
	ClientID    string
	TokenHash   string
	Scopes      []string
	RotatedFrom string
	UsedAt      *time.Time
	ExpiresAt   time.Time
}
