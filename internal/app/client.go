package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// ClientByRealmAndClientID looks a client up by its (realm UUID, client_id)
// natural key — the unique pair the schema enforces.
func ClientByRealmAndClientID(realmID, clientID string) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.ClientID, clientID),
	)
}

// ClientRepository persists OIDC clients via the kit generics.
type ClientRepository interface {
	repository.Creator[domain.Client]
	repository.Getter[domain.Client]
	repository.Lister[domain.Client]
	repository.Deleter
}

// ClientUsecase is the admin surface for OIDC clients. Get/List/Delete are
// plain kit generics; Create is overridden to mint a confidential client's
// secret (hashed at rest, raw surfaced once).
type ClientUsecase interface {
	repository.Creator[domain.Client]
	repository.Getter[domain.Client]
	repository.Lister[domain.Client]
	repository.Deleter
}

type clientUsecase struct {
	usecase.Getter[domain.Client]
	usecase.Lister[domain.Client]
	repository.Deleter

	repo ClientRepository
}

func NewClientUsecase(repo ClientRepository) ClientUsecase {
	return &clientUsecase{
		Getter:  usecase.NewGetter(repo, domain.ResourceTypeClient),
		Lister:  usecase.NewLister(repo),
		Deleter: usecase.NewDeleter(repo),
		repo:    repo,
	}
}

func (uc *clientUsecase) Create(ctx context.Context, c domain.Client) (domain.Client, error) {
	if c.RealmID() == "" {
		return nil, apierrors.InvalidArgument("realm_id is required")
	}
	if c.ClientID() == "" {
		return nil, apierrors.InvalidArgument("client_id is required")
	}
	if !c.ClientType().Valid() {
		return nil, apierrors.InvalidArgument("invalid client type")
	}

	if c.ClientType() != domain.ClientTypeConfidential {
		return uc.repo.Create(ctx, c)
	}

	// Confidential client: mint a secret, store only its hash, surface the
	// raw value exactly once on the returned resource.
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return nil, apierrors.InternalError("failed to generate client secret")
	}
	created, err := uc.repo.Create(ctx, withClientSecretHash(c, hash))
	if err != nil {
		return nil, err
	}
	return clientWithSecret{Client: created, secret: raw}, nil
}

// withClientSecretHash rebuilds the create input carrying the secret hash;
// id/timestamps stay server-assigned.
func withClientSecretHash(c domain.Client, hash string) domain.Client {
	return domain.NewClient(c.RealmID(), c.ClientID(), c.ClientType(), c.Name(),
		domain.WithClientGrantTypes(c.GrantTypes()),
		domain.WithClientScopes(c.Scopes()),
		domain.WithClientRedirectURIs(c.RedirectURIs()),
		domain.WithClientPKCERequired(c.PKCERequired()),
		domain.WithClientSecretHash(hash),
	)
}

// clientWithSecret decorates the persisted client with its one-time raw
// secret so the create response can return it without persisting it.
type clientWithSecret struct {
	domain.Client
	secret string
}

func (c clientWithSecret) Secret() string { return c.secret }
