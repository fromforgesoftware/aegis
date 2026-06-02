package app

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// ExternalUser is the verified user the upstream IdP asserts.
type ExternalUser struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
}

// Connector verifies a raw upstream credential against an IdP config and
// returns the asserted user. Each kind (Firebase, Google, custom OIDC…) is
// implemented as a distinct Connector wired into a Connectors map.
type Connector interface {
	Kind() domain.ExternalIDPKind
	Verify(ctx context.Context, cfg domain.ExternalIDPConfig, rawToken string) (ExternalUser, error)
}

// Connectors maps IdP kinds to their connectors; built from the registered
// fx value-group at startup.
type Connectors map[domain.ExternalIDPKind]Connector

func NewConnectors(cs []Connector) Connectors {
	out := Connectors{}
	for _, c := range cs {
		out[c.Kind()] = c
	}
	return out
}

// AccountExternalIDRepository persists the account ↔ upstream-identity links.
type AccountExternalIDRepository interface {
	Create(ctx context.Context, link domain.AccountExternalID) error
	GetByExternalID(ctx context.Context, kind domain.ExternalIDPKind, externalID string) (domain.AccountExternalID, error)
	ListByAccount(ctx context.Context, accountID string, opts ...search.Option) ([]domain.AccountExternalID, error)
}

// ResolveAccountInput is the federation entrypoint: a raw upstream token plus
// the realm + IdP-config name selecting which connector verifies it.
type ResolveAccountInput struct {
	RealmID  string
	IDPName  string
	RawToken string
}

// ResolveAccountResult records what happened to the upstream identity. One of
// three terminal states:
//   - Created=false, LinkRequired=false : an existing link resolved an account.
//   - Created=true,  LinkRequired=false : JIT-provisioned a new account.
//   - Created=false, LinkRequired=true  : the upstream email matches an
//     existing account but it isn't safe to auto-link (the upstream didn't
//     prove email ownership); the caller runs an explicit linking flow.
type ResolveAccountResult struct {
	Account      domain.Account
	Created      bool
	LinkRequired bool
}

// IdentityBrokerUsecase verifies an upstream identity and resolves (or
// provisions) the Aegis account it maps to.
type IdentityBrokerUsecase interface {
	ResolveAccount(ctx context.Context, in ResolveAccountInput) (ResolveAccountResult, error)
}

type identityBrokerUsecase struct {
	idps       ExternalIDPConfigRepository
	accounts   AccountRepository
	links      AccountExternalIDRepository
	connectors Connectors
	tx         persistence.Transactioner
}

func NewIdentityBrokerUsecase(
	idps ExternalIDPConfigRepository,
	accounts AccountRepository,
	links AccountExternalIDRepository,
	connectors Connectors,
	tx persistence.Transactioner,
) IdentityBrokerUsecase {
	return &identityBrokerUsecase{idps: idps, accounts: accounts, links: links, connectors: connectors, tx: tx}
}

func (uc *identityBrokerUsecase) ResolveAccount(ctx context.Context, in ResolveAccountInput) (ResolveAccountResult, error) {
	if in.RealmID == "" || in.IDPName == "" || in.RawToken == "" {
		return ResolveAccountResult{}, apierrors.InvalidArgument("realm_id, idp_name and token are required")
	}

	cfg, err := uc.idps.Get(ctx, byRealmIDPName(in.RealmID, in.IDPName))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return ResolveAccountResult{}, apierrors.InvalidArgument("unknown IdP")
		}
		return ResolveAccountResult{}, err
	}
	if !cfg.Enabled() {
		return ResolveAccountResult{}, apierrors.InvalidArgument("IdP is disabled")
	}

	conn, ok := uc.connectors[cfg.Kind()]
	if !ok {
		return ResolveAccountResult{}, apierrors.InternalError("no connector registered for this IdP kind")
	}

	user, err := conn.Verify(ctx, cfg, in.RawToken)
	if err != nil {
		return ResolveAccountResult{}, err
	}
	if user.ID == "" {
		return ResolveAccountResult{}, apierrors.Unauthenticated("upstream identity has no subject")
	}

	link, err := uc.links.GetByExternalID(ctx, cfg.Kind(), user.ID)
	if err == nil {
		acc, err := uc.accounts.Get(ctx, byID(link.AccountID))
		if err != nil {
			return ResolveAccountResult{}, err
		}
		return ResolveAccountResult{Account: acc, Created: false}, nil
	}
	if !apierrors.Is(err, apierrors.CodeNotFound) {
		return ResolveAccountResult{}, err
	}

	// New upstream identity: check for an existing account on the same realm
	// + email before JIT-creating. If found, auto-link only when the upstream
	// proved email ownership; otherwise hand back a LinkRequired result so
	// the caller can run an explicit linking flow.
	if user.Email != "" {
		existing, err := uc.accounts.Get(ctx, byRealmEmail(in.RealmID, normalizeEmail(user.Email)))
		if err == nil {
			if !user.EmailVerified {
				return ResolveAccountResult{Account: existing, LinkRequired: true}, nil
			}
			if err := uc.links.Create(ctx, domain.AccountExternalID{
				AccountID: existing.ID(), Kind: cfg.Kind(), ExternalID: user.ID,
			}); err != nil {
				return ResolveAccountResult{}, err
			}
			return ResolveAccountResult{Account: existing, Created: false}, nil
		}
		if !apierrors.Is(err, apierrors.CodeNotFound) {
			return ResolveAccountResult{}, err
		}
	}

	return uc.provision(ctx, in.RealmID, cfg.Kind(), user)
}

// provision JIT-creates an account + link in one transaction so a half-failed
// federation can't leave an unlinked account behind.
func (uc *identityBrokerUsecase) provision(ctx context.Context, realmID string, kind domain.ExternalIDPKind, user ExternalUser) (ResolveAccountResult, error) {
	acc := domain.NewAccount(realmID, normalizeEmail(user.Email), user.Name,
		domain.WithAccountType(domain.AccountTypeUser),
		domain.WithAccountStatus(domain.AccountStatusEnabled),
		domain.WithAccountEmailVerified(user.EmailVerified),
	)
	var out domain.Account
	err := uc.tx.Exec(ctx, func(ctx context.Context) error {
		created, err := uc.accounts.Create(ctx, acc)
		if err != nil {
			return err
		}
		if err := uc.links.Create(ctx, domain.AccountExternalID{
			AccountID:  created.ID(),
			Kind:       kind,
			ExternalID: user.ID,
		}); err != nil {
			return err
		}
		out = created
		return nil
	})
	if err != nil {
		return ResolveAccountResult{}, err
	}
	return ResolveAccountResult{Account: out, Created: true}, nil
}

func byRealmIDPName(realmID, name string) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Name, name),
	)
}
