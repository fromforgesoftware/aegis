package app_test

import (
	"context"
	"encoding/json"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func testKeyCipher(t *testing.T) app.KeyCipher {
	c, err := cryptox.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	return c
}

func mustJWK(t *testing.T, kid string) json.RawMessage {
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	jwk, err := cryptox.PublicJWK(&key.PublicKey, kid)
	require.NoError(t, err)
	return jwk
}

// echoCreatedKey makes the repo Create mock return exactly the key it was
// handed, so the service's generated (real) key flows back through.
func echoCreatedKey(_ context.Context, k domain.SigningKey) (domain.SigningKey, error) {
	return k, nil
}

func TestActiveSigner_GeneratesOnFirstUse(t *testing.T) {
	keys := apptest.NewSigningKeyRepository(t)
	keys.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("signing key", "r"))
	want := domain.NewSigningKey("r", "", "RS256", nil, nil)
	keys.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchSigningKey(want))).
		RunAndReturn(echoCreatedKey)

	svc := app.NewSigningKeyService(keys, testKeyCipher(t))
	signer, err := svc.ActiveSigner(context.Background(), "r")
	require.NoError(t, err)
	assert.NotEmpty(t, signer.Kid)
	require.NotNil(t, signer.Key)
}

func TestActiveSigner_UsesExistingKey(t *testing.T) {
	cipher := testKeyCipher(t)
	key, err := cryptox.GenerateRSAKey()
	require.NoError(t, err)
	der, err := cryptox.MarshalPrivateKey(key)
	require.NoError(t, err)
	sealed, err := cipher.Seal(der)
	require.NoError(t, err)
	jwk, err := cryptox.PublicJWK(&key.PublicKey, "kid-existing")
	require.NoError(t, err)

	keys := apptest.NewSigningKeyRepository(t)
	keys.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewSigningKey("r", "kid-existing", "RS256", jwk, sealed), nil)

	svc := app.NewSigningKeyService(keys, cipher)
	signer, err := svc.ActiveSigner(context.Background(), "r")
	require.NoError(t, err)
	assert.Equal(t, "kid-existing", signer.Kid)
	require.NotNil(t, signer.Key)
	assert.Equal(t, key.N, signer.Key.N, "opened key must match the sealed one")
}

func TestJWKS_GeneratesWhenEmpty(t *testing.T) {
	keys := apptest.NewSigningKeyRepository(t)
	keys.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse([]domain.SigningKey{}, 0), nil)
	keys.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(echoCreatedKey)

	svc := app.NewSigningKeyService(keys, testKeyCipher(t))
	jwks, err := svc.JWKS(context.Background(), "r")
	require.NoError(t, err)
	s := string(jwks)
	assert.Contains(t, s, `"keys"`)
	assert.Contains(t, s, `"kty":"RSA"`)
}

func TestJWKS_ListsPublishedKeys(t *testing.T) {
	keys := apptest.NewSigningKeyRepository(t)
	keys.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.SigningKey{
		domain.NewSigningKey("r", "kid-1", "RS256", mustJWK(t, "kid-1"), nil),
		domain.NewSigningKey("r", "kid-2", "RS256", mustJWK(t, "kid-2"), nil),
	}, 2), nil)

	svc := app.NewSigningKeyService(keys, testKeyCipher(t))
	jwks, err := svc.JWKS(context.Background(), "r")
	require.NoError(t, err)
	s := string(jwks)
	assert.Contains(t, s, "kid-1")
	assert.Contains(t, s, "kid-2")
}
