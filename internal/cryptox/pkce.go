package cryptox

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// PKCEChallengeS256 computes the RFC 7636 S256 code challenge for a verifier:
// base64url(sha256(verifier)), no padding.
func PKCEChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// VerifyPKCES256 reports whether verifier matches the stored S256 challenge,
// in constant time.
func VerifyPKCES256(verifier, challenge string) bool {
	want := PKCEChallengeS256(verifier)
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}
