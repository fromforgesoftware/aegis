package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newBroker(t *testing.T, conn app.Connector) (
	*apptest.ExternalIDPConfigRepository,
	*apptest.AccountRepository,
	*apptest.AccountExternalIDRepository,
	app.IdentityBrokerUsecase,
) {
	idps := apptest.NewExternalIDPConfigRepository(t)
	accounts := apptest.NewAccountRepository(t)
	links := apptest.NewAccountExternalIDRepository(t)
	uc := app.NewIdentityBrokerUsecase(idps, accounts, links,
		app.NewConnectors([]app.Connector{conn}), persistencetest.NewTransactioner())
	return idps, accounts, links, uc
}

func googleConfig() domain.ExternalIDPConfig {
	return internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
		internaltest.WithExternalIDPName("google-prod"),
	)
}

func TestResolveAccount_ExistingLink(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, accounts, links, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(googleConfig(), nil)
	conn.EXPECT().Verify(mock.Anything, mock.Anything, "raw-id-token").
		Return(app.ExternalUser{ID: "google-uid", Email: "a@b.com", EmailVerified: true, Name: "A"}, nil)
	links.EXPECT().GetByExternalID(mock.Anything, domain.ExternalIDPKindOAuthGoogle, "google-uid").
		Return(domain.AccountExternalID{AccountID: "acc-1", Kind: domain.ExternalIDPKindOAuthGoogle, ExternalID: "google-uid"}, nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1"), internaltest.WithAccountEmail("a@b.com")), nil)

	res, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "raw-id-token",
	})
	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.Equal(t, "acc-1", res.Account.ID())
}

func TestResolveAccount_JITProvisions(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, accounts, links, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(googleConfig(), nil)
	conn.EXPECT().Verify(mock.Anything, mock.Anything, "raw-id-token").
		Return(app.ExternalUser{ID: "google-uid", Email: "Trader@Example.com", EmailVerified: true, Name: "Trader"}, nil)
	links.EXPECT().GetByExternalID(mock.Anything, domain.ExternalIDPKindOAuthGoogle, "google-uid").
		Return(domain.AccountExternalID{}, apierrors.NotFound("account_external_id", "google-uid"))
	// Email-collision probe runs before JIT: no existing account on (realm, email).
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("account", "trader@example.com")).Once()
	accounts.EXPECT().Create(mock.Anything, mock.MatchedBy(func(a domain.Account) bool {
		// JIT normalises the email and marks it verified per the upstream claim.
		return a.RealmID() == "r" && a.Email() == "trader@example.com" && a.EmailVerified()
	})).Return(internaltest.NewAccount(
		internaltest.WithAccountID("acc-new"),
		internaltest.WithAccountEmail("trader@example.com"),
		internaltest.WithAccountEmailVerified(true),
	), nil)
	links.EXPECT().Create(mock.Anything, domain.AccountExternalID{
		AccountID: "acc-new", Kind: domain.ExternalIDPKindOAuthGoogle, ExternalID: "google-uid",
	}).Return(nil)

	res, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "raw-id-token",
	})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, "acc-new", res.Account.ID())
}

func TestResolveAccount_VerifiedEmailAutoLinks(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, accounts, links, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(googleConfig(), nil)
	conn.EXPECT().Verify(mock.Anything, mock.Anything, "raw-id-token").
		Return(app.ExternalUser{ID: "google-uid", Email: "alice@example.com", EmailVerified: true, Name: "Alice"}, nil)
	links.EXPECT().GetByExternalID(mock.Anything, domain.ExternalIDPKindOAuthGoogle, "google-uid").
		Return(domain.AccountExternalID{}, apierrors.NotFound("account_external_id", "google-uid"))
	// A pre-existing password account in the same realm with the matching email.
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(
		internaltest.NewAccount(
			internaltest.WithAccountID("acc-existing"),
			internaltest.WithAccountEmail("alice@example.com"),
		), nil)
	// Upstream proved email ownership ⇒ auto-link, no JIT account.
	links.EXPECT().Create(mock.Anything, domain.AccountExternalID{
		AccountID: "acc-existing", Kind: domain.ExternalIDPKindOAuthGoogle, ExternalID: "google-uid",
	}).Return(nil)

	res, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "raw-id-token",
	})
	require.NoError(t, err)
	assert.False(t, res.Created, "no new account; existing one reused")
	assert.False(t, res.LinkRequired)
	assert.Equal(t, "acc-existing", res.Account.ID())
}

func TestResolveAccount_UnverifiedEmailRequiresLink(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGitHub)

	idps, accounts, links, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGitHub),
		internaltest.WithExternalIDPName("github-prod"),
	), nil)
	conn.EXPECT().Verify(mock.Anything, mock.Anything, "ghp_TOKEN").
		Return(app.ExternalUser{ID: "42", Email: "alice@example.com", EmailVerified: false, Name: "Alice"}, nil)
	links.EXPECT().GetByExternalID(mock.Anything, domain.ExternalIDPKindOAuthGitHub, "42").
		Return(domain.AccountExternalID{}, apierrors.NotFound("account_external_id", "42"))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(
		internaltest.NewAccount(
			internaltest.WithAccountID("acc-existing"),
			internaltest.WithAccountEmail("alice@example.com"),
		), nil)

	res, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "github-prod", RawToken: "ghp_TOKEN",
	})
	require.NoError(t, err)
	assert.True(t, res.LinkRequired, "upstream didn't prove email ownership — explicit linking required")
	assert.False(t, res.Created)
	assert.Equal(t, "acc-existing", res.Account.ID())
}

func TestResolveAccount_UnknownIDP(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, _, _, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("external_idp_config", "google-prod"))

	_, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "x",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestResolveAccount_DisabledIDP(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, _, _, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGoogle),
		internaltest.WithExternalIDPName("google-prod"),
		internaltest.WithExternalIDPEnabled(false),
	), nil)

	_, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "x",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestResolveAccount_NoConnectorForKind(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, _, _, uc := newBroker(t, conn)
	// Config is for GitHub but only the Google connector is registered.
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(internaltest.NewExternalIDP(
		internaltest.WithExternalIDPRealmID("r"),
		internaltest.WithExternalIDPKind(domain.ExternalIDPKindOAuthGitHub),
		internaltest.WithExternalIDPName("github-prod"),
	), nil)

	_, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "github-prod", RawToken: "x",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInternalError))
}

func TestResolveAccount_ConnectorRejects(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	idps, _, _, uc := newBroker(t, conn)
	idps.EXPECT().Get(mock.Anything, mock.Anything).Return(googleConfig(), nil)
	conn.EXPECT().Verify(mock.Anything, mock.Anything, "bogus").
		Return(app.ExternalUser{}, apierrors.Unauthenticated("invalid id_token"))

	_, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "bogus",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestResolveAccount_Validates(t *testing.T) {
	conn := apptest.NewConnector(t)
	conn.EXPECT().Kind().Return(domain.ExternalIDPKindOAuthGoogle)

	_, _, _, uc := newBroker(t, conn)
	_, err := uc.ResolveAccount(context.Background(), app.ResolveAccountInput{RealmID: "r"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
