package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// newOpaqueToken returns a high-entropy URL-safe token and its SHA-256
// hash. The raw token is delivered to the user (e.g. an email link); only
// the hash is persisted, so a database leak can't yield usable tokens.
// SHA-256 is sufficient here (unlike passwords) because the token is
// already high-entropy random, not user-chosen.
func newOpaqueToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
