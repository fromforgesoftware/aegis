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

// RealmByName filters realms by their unique name — the slug carried in the
// OAuth/OIDC URL path (/realms/{name}/...). Realm-scoped rows key on the realm
// UUID, so the protocol controllers resolve the path name to an id via this.
func RealmByName(name string) search.Option {
	return search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.Name, name))
}

// RealmRepository persists realms via kit generics.
type RealmRepository interface {
	repository.Creator[domain.Realm]
	repository.Getter[domain.Realm]
	repository.Lister[domain.Realm]
	repository.Patcher[domain.Realm]
	repository.Deleter
}

// RealmUsecase is the admin surface for realms.
type RealmUsecase interface {
	repository.Getter[domain.Realm]
	repository.Lister[domain.Realm]
	repository.Patcher[domain.Realm]
	repository.Deleter
	Create(ctx context.Context, realm domain.Realm) (domain.Realm, error)
}

type realmUsecase struct {
	usecase.Getter[domain.Realm]
	usecase.Lister[domain.Realm]
	repository.Patcher[domain.Realm]
	repository.Deleter

	repo RealmRepository
}

func NewRealmUsecase(repo RealmRepository) RealmUsecase {
	return &realmUsecase{
		Getter:  usecase.NewGetter(repo, domain.ResourceTypeRealm),
		Lister:  usecase.NewLister(repo),
		Patcher: repo,
		Deleter: usecase.NewDeleter(repo),
		repo:    repo,
	}
}

func (uc *realmUsecase) Create(ctx context.Context, realm domain.Realm) (domain.Realm, error) {
	if realm.Name() == "" {
		return nil, apierrors.InvalidArgument("name is required")
	}
	return uc.repo.Create(ctx, realm)
}
