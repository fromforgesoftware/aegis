package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func echoCreatedClient(_ context.Context, c domain.Client) (domain.Client, error) { return c, nil }

func TestClientCreate_ConfidentialMintsSecret(t *testing.T) {
	repo := apptest.NewClientRepository(t)
	want := domain.NewClient("r", "web", domain.ClientTypeConfidential, "Web App")
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchClient(want))).
		RunAndReturn(echoCreatedClient)

	uc := app.NewClientUsecase(repo)
	got, err := uc.Create(context.Background(),
		domain.NewClient("r", "web", domain.ClientTypeConfidential, "Web App"))
	require.NoError(t, err)
	assert.NotEmpty(t, got.Secret(), "confidential client returns its secret once")
	assert.NotEmpty(t, got.SecretHash(), "the hash is what gets persisted")
}

func TestClientCreate_PublicHasNoSecret(t *testing.T) {
	repo := apptest.NewClientRepository(t)
	want := domain.NewClient("r", "spa", domain.ClientTypePublic, "SPA")
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchClient(want))).
		RunAndReturn(echoCreatedClient)

	uc := app.NewClientUsecase(repo)
	got, err := uc.Create(context.Background(),
		domain.NewClient("r", "spa", domain.ClientTypePublic, "SPA"))
	require.NoError(t, err)
	assert.Empty(t, got.Secret())
	assert.Empty(t, got.SecretHash())
}

func TestClientCreate_Validation(t *testing.T) {
	cases := map[string]domain.Client{
		"empty realm":     domain.NewClient("", "web", domain.ClientTypePublic, "X"),
		"empty client_id": domain.NewClient("r", "", domain.ClientTypePublic, "X"),
		"invalid type":    domain.NewClient("r", "web", domain.ClientType("BOGUS"), "X"),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			// Validation fails before any repo call.
			uc := app.NewClientUsecase(apptest.NewClientRepository(t))
			_, err := uc.Create(context.Background(), in)
			require.Error(t, err)
			assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)
		})
	}
}
