package cryptox_test

import (
	"encoding/base32"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/cryptox"
)

// RFC 6238 SHA1 test seed "12345678901234567890"; at T=59s the 8-digit OTP is
// 94287082, so the 6-digit code is its last six digits, 287082.
func TestValidateTOTP_RFC6238Vector(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString([]byte("12345678901234567890"))

	assert.True(t, cryptox.ValidateTOTP(secret, "287082", time.Unix(59, 0)))
	assert.False(t, cryptox.ValidateTOTP(secret, "000000", time.Unix(59, 0)))
}

func TestValidateTOTP_AcceptsClockSkew(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString([]byte("12345678901234567890"))
	// The T=59 code stays valid one step later (T≈89) within the ±1 window.
	assert.True(t, cryptox.ValidateTOTP(secret, "287082", time.Unix(80, 0)))
}

func TestGenerateTOTPSecret_RoundTrips(t *testing.T) {
	secret, err := cryptox.GenerateTOTPSecret()
	require.NoError(t, err)
	require.NotEmpty(t, secret)

	uri := cryptox.TOTPProvisioningURI(secret, "Aegis", "user@example.com")
	assert.Contains(t, uri, "otpauth://totp/")
	assert.Contains(t, uri, "secret="+secret)
}
