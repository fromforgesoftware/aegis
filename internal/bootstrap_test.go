package internal

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func testBootstrapConfig() bootstrapConfig {
	return bootstrapConfig{
		realm:            "master",
		realmDisplayName: "Master",
		adminEmail:       "admin@forge.local",
		adminPassword:    "s3cret-pw",
		adminName:        "Administrator",
		clientID:         "foundry",
		redirectURIs:     []string{"http://localhost:8080/auth/callback"},
	}
}

func TestEnsureBootstrap_CreatesRealmAdminAndClient(t *testing.T) {
	realms := apptest.NewRealmUsecase(t)
	clients := apptest.NewClientUsecase(t)
	authx := apptest.NewAuthxUsecase(t)

	realms.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("realm", "master"))
	realms.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.Realm) bool {
		return r.Name() == "master"
	})).Return(domain.NewRealm("master", domain.WithRealmID("realm-1")), nil)

	authx.EXPECT().Register(mock.Anything, mock.MatchedBy(func(in app.RegisterInput) bool {
		return in.RealmID == "realm-1" && in.Email == "admin@forge.local" && in.Password == "s3cret-pw"
	})).Return(nil, nil)

	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("client", "foundry"))
	clients.EXPECT().Create(mock.Anything, mock.MatchedBy(func(c domain.Client) bool {
		return c.ClientID() == "foundry" && c.RealmID() == "realm-1" &&
			c.ClientType() == domain.ClientTypePublic && c.PKCERequired()
	})).Return(domain.NewClient("realm-1", "foundry", domain.ClientTypePublic, "foundry"), nil)

	require.NoError(t, ensureBootstrap(context.Background(), testBootstrapConfig(), realms, clients, authx, logger.New()))
}

func TestEnsureBootstrap_IdempotentWhenAllExist(t *testing.T) {
	realms := apptest.NewRealmUsecase(t)
	clients := apptest.NewClientUsecase(t)
	authx := apptest.NewAuthxUsecase(t)

	// Realm + client already present; admin re-registration reports AlreadyExists.
	// No Create calls expected — the mocks fail the test if any fire.
	realms.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewRealm("master", domain.WithRealmID("realm-1")), nil)
	authx.EXPECT().Register(mock.Anything, mock.Anything).
		Return(nil, apierrors.AlreadyExists("account", "admin@forge.local"))
	clients.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewClient("realm-1", "foundry", domain.ClientTypePublic, "foundry"), nil)

	require.NoError(t, ensureBootstrap(context.Background(), testBootstrapConfig(), realms, clients, authx, logger.New()))
}
