package app_test

import (
	"bytes"
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newCipher(t *testing.T) *cryptox.Cipher {
	t.Helper()
	c, err := cryptox.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	return c
}

func echoCreatedIDP(_ context.Context, c domain.ExternalIDPConfig) (domain.ExternalIDPConfig, error) {
	return c, nil
}

func TestExternalIDPCreateWithSecret_Seals(t *testing.T) {
	repo := apptest.NewExternalIDPConfigRepository(t)
	cipher := newCipher(t)
	want := internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
		internaltest.WithExternalIDPName("google-prod"),
	)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchExternalIDP(want))).
		RunAndReturn(echoCreatedIDP)

	uc := app.NewExternalIDPConfigUsecase(repo, cipher)
	got, err := uc.CreateWithSecret(context.Background(),
		internaltest.NewExternalIDP(
			internaltest.WithExternalIDPRealmID("r"),
			internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
			internaltest.WithExternalIDPName("google-prod"),
			internaltest.WithExternalIDPClientID("client-1"),
		),
		"upstream-secret",
	)
	require.NoError(t, err)
	require.NotEmpty(t, got.ClientSecretEncrypted(), "secret was sealed onto the persisted config")

	plain, err := cipher.Open(got.ClientSecretEncrypted())
	require.NoError(t, err)
	assert.True(t, bytes.Equal(plain, []byte("upstream-secret")), "the sealed secret round-trips through the same cipher")
}

func TestExternalIDPCreateWithSecret_EmptyLeavesNil(t *testing.T) {
	repo := apptest.NewExternalIDPConfigRepository(t)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(c domain.ExternalIDPConfig) bool {
		return c.ClientSecretEncrypted() == nil
	})).RunAndReturn(echoCreatedIDP)

	uc := app.NewExternalIDPConfigUsecase(repo, newCipher(t))
	_, err := uc.CreateWithSecret(context.Background(),
		internaltest.NewExternalIDP(internaltest.WithExternalIDPRealmID("r")),
		"",
	)
	require.NoError(t, err)
}

func TestExternalIDPCreate_Validation(t *testing.T) {
	cases := map[string]domain.ExternalIDPConfig{
		"empty realm": domain.NewExternalIDPConfig("", domain.ExternalIDPKindOAuthGoogle, "x"),
		"empty name":  domain.NewExternalIDPConfig("r", domain.ExternalIDPKindOAuthGoogle, ""),
		"bad kind":    domain.NewExternalIDPConfig("r", domain.ExternalIDPKind("BOGUS"), "x"),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			uc := app.NewExternalIDPConfigUsecase(apptest.NewExternalIDPConfigRepository(t), newCipher(t))
			_, err := uc.Create(context.Background(), in)
			require.Error(t, err)
			assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
		})
	}
}
