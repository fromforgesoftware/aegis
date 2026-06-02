package cryptox

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// totpPeriod is the RFC 6238 time step; totpDigits the code length.
const (
	totpPeriod = 30 * time.Second
	totpDigits = 6
)

var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret returns a fresh base32-encoded shared secret (160 bits).
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32NoPad.EncodeToString(buf), nil
}

// TOTPProvisioningURI builds the otpauth:// URI an authenticator app scans.
func TOTPProvisioningURI(secret, issuer, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", totpDigits))
	q.Set("period", fmt.Sprintf("%d", int(totpPeriod.Seconds())))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// GenerateTOTPCode returns the current code for secret at t. Primarily useful
// for tests and tooling; verification uses ValidateTOTP.
func GenerateTOTPCode(secret string, t time.Time) (string, error) {
	key, err := base32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", err
	}
	return hotp(key, uint64(t.Unix()/int64(totpPeriod.Seconds()))), nil
}

// ValidateTOTP reports whether code is valid for secret at t, accepting one
// step of clock skew on either side.
func ValidateTOTP(secret, code string, t time.Time) bool {
	key, err := base32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return false
	}
	code = strings.TrimSpace(code)
	counter := uint64(t.Unix() / int64(totpPeriod.Seconds()))
	for _, c := range []uint64{counter - 1, counter, counter + 1} {
		if hmac.Equal([]byte(hotp(key, c)), []byte(code)) {
			return true
		}
	}
	return false
}

// hotp is the RFC 4226 HMAC-SHA1 one-time-password for a counter.
func hotp(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	mod := uint32(1)
	for range totpDigits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, value%mod)
}
