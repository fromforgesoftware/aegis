package internal

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/resource"
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

func testAdmin() domain.Account {
	return domain.NewAccount("realm-1", "admin@forge.local", "Administrator", domain.WithAccountID("acct-1"))
}

func TestEnsureBootstrap_CreatesRealmAdminAndClient(t *testing.T) {
	realms := apptest.NewRealmUsecase(t)
	clients := apptest.NewClientUsecase(t)
	authx := apptest.NewAuthxUsecase(t)
	orgs := apptest.NewOrganizationUsecase(t)

	realms.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("realm", "master"))
	realms.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.Realm) bool {
		return r.Name() == "master"
	})).Return(domain.NewRealm("master", domain.WithRealmID("realm-1")), nil)

	authx.EXPECT().Register(mock.Anything, mock.MatchedBy(func(in app.RegisterInput) bool {
		return in.RealmID == "realm-1" && in.Email == "admin@forge.local" && in.Password == "s3cret-pw"
	})).Return(testAdmin(), nil)

	clients.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("client", "foundry"))
	clients.EXPECT().Create(mock.Anything, mock.MatchedBy(func(c domain.Client) bool {
		return c.ClientID() == "foundry" && c.RealmID() == "realm-1" &&
			c.ClientType() == domain.ClientTypePublic && c.PKCERequired()
	})).Return(domain.NewClient("realm-1", "foundry", domain.ClientTypePublic, "foundry"), nil)

	require.NoError(t, ensureBootstrap(context.Background(), testBootstrapConfig(), realms, clients, authx, orgs, logger.New()))
}

func TestEnsureBootstrap_IdempotentWhenAllExist(t *testing.T) {
	realms := apptest.NewRealmUsecase(t)
	clients := apptest.NewClientUsecase(t)
	authx := apptest.NewAuthxUsecase(t)
	orgs := apptest.NewOrganizationUsecase(t)

	// Realm + client already present; admin re-registration reports AlreadyExists
	// and the admin id is recovered via login. No Create calls expected — the
	// mocks fail the test if any fire.
	realms.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewRealm("master", domain.WithRealmID("realm-1")), nil)
	authx.EXPECT().Register(mock.Anything, mock.Anything).
		Return(nil, apierrors.AlreadyExists("account", "admin@forge.local"))
	authx.EXPECT().Login(mock.Anything, mock.MatchedBy(func(in app.LoginInput) bool {
		return in.RealmID == "realm-1" && in.Email == "admin@forge.local" && in.Password == "s3cret-pw"
	})).Return(testAdmin(), nil)
	clients.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewClient("realm-1", "foundry", domain.ClientTypePublic, "foundry"), nil)

	require.NoError(t, ensureBootstrap(context.Background(), testBootstrapConfig(), realms, clients, authx, orgs, logger.New()))
}

func TestEnsureOrgs_SeedsAndActivatesPrimary(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	cfg := testBootstrapConfig()
	cfg.orgs = []bootstrapOrg{{name: "Acme", slug: "acme"}, {name: "Globex", slug: "globex"}}

	// Neither org exists yet.
	orgs.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse[domain.Organization](nil, 0), nil).Twice()
	orgs.EXPECT().Create(mock.Anything, mock.MatchedBy(func(o domain.Organization) bool {
		return o.Slug() == "acme" && o.Owner() != nil && o.Owner().ID() == "acct-1"
	})).Return(domain.NewOrganization("realm-1", "Acme", "acme", domain.WithOrganizationID("org-acme")), nil)
	orgs.EXPECT().Create(mock.Anything, mock.MatchedBy(func(o domain.Organization) bool {
		return o.Slug() == "globex"
	})).Return(domain.NewOrganization("realm-1", "Globex", "globex", domain.WithOrganizationID("org-globex")), nil)

	// No active org stored — the primary (first) gets activated, only once.
	orgs.EXPECT().ActiveOrg(mock.Anything, "acct-1").Return("", "", nil)
	orgs.EXPECT().Activate(mock.Anything, "acct-1", "org-acme").Return(nil)

	require.NoError(t, ensureOrgs(context.Background(), cfg, "realm-1", "acct-1", orgs, logger.New()))
}

func TestEnsureOrgs_IdempotentAndKeepsActiveChoice(t *testing.T) {
	orgs := apptest.NewOrganizationUsecase(t)
	cfg := testBootstrapConfig()
	cfg.orgs = []bootstrapOrg{{name: "Acme", slug: "acme"}}

	// Org already seeded and the admin has an active org — nothing to do.
	orgs.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse([]domain.Organization{
			domain.NewOrganization("realm-1", "Acme", "acme", domain.WithOrganizationID("org-acme")),
		}, 1), nil)
	orgs.EXPECT().ActiveOrg(mock.Anything, "acct-1").Return("org-globex", "OWNER", nil)

	require.NoError(t, ensureOrgs(context.Background(), cfg, "realm-1", "acct-1", orgs, logger.New()))
}

func TestParseBootstrapOrgs(t *testing.T) {
	got, err := parseBootstrapOrgs(`[{"name":"Acme","slug":"acme"},{"slug":"globex"}]`)
	require.NoError(t, err)
	require.Equal(t, []bootstrapOrg{{name: "Acme", slug: "acme"}, {name: "globex", slug: "globex"}}, got)

	got, err = parseBootstrapOrgs("")
	require.NoError(t, err)
	require.Empty(t, got)

	_, err = parseBootstrapOrgs(`[{"name":"NoSlug"}]`)
	require.Error(t, err)

	_, err = parseBootstrapOrgs(`Acme:acme,Globex:globex`)
	require.Error(t, err)

	_, err = parseBootstrapOrgs(`[not-json`)
	require.Error(t, err)
}
