package cryptox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/cryptox"
)

func TestCipher_SealOpenRoundtrip(t *testing.T) {
	c, err := cryptox.NewCipher(make([]byte, 32))
	require.NoError(t, err)

	plaintext := []byte("a private signing key's PKCS#8 bytes")
	sealed, err := c.Seal(plaintext)
	require.NoError(t, err)
	assert.NotContains(t, string(sealed), "private signing key", "sealed blob must not contain plaintext")

	opened, err := c.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, plaintext, opened)
}

func TestCipher_OpenWithWrongKEKFails(t *testing.T) {
	a, _ := cryptox.NewCipher(make([]byte, 32))
	otherKEK := make([]byte, 32)
	otherKEK[0] = 0x01
	b, _ := cryptox.NewCipher(otherKEK)

	sealed, err := a.Seal([]byte("secret"))
	require.NoError(t, err)
	_, err = b.Open(sealed)
	require.Error(t, err)
}

func TestNewCipher_RejectsWrongKEKLength(t *testing.T) {
	_, err := cryptox.NewCipher(make([]byte, 16))
	require.Error(t, err)
}

func TestRSAKeyRoundtrip(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)

	der, err := cryptox.MarshalPrivateKey(key)
	require.NoError(t, err)
	got, err := cryptox.ParsePrivateKey(der)
	require.NoError(t, err)
	assert.Equal(t, key.N, got.N)
}

func TestPublicJWK(t *testing.T) {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)

	jwk, err := cryptox.PublicJWK(&key.PublicKey, "kid-1")
	require.NoError(t, err)
	s := string(jwk)
	assert.Contains(t, s, `"kid":"kid-1"`)
	assert.Contains(t, s, `"kty":"RSA"`)
	assert.Contains(t, s, `"alg":"RS256"`)
}
