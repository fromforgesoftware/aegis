package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// AuthzResourceRepository persists registered authz resources via kit generics.
type AuthzResourceRepository interface {
	repository.Creator[domain.AuthzResource]
	repository.Getter[domain.AuthzResource]
	repository.Lister[domain.AuthzResource]
	repository.Patcher[domain.AuthzResource]
	repository.Deleter
}

// AuthzResourceUsecase is the consumer's seam for registering domain objects
// into the authz hierarchy. Create validates the parent-realm invariant: a
// resource can only point to a parent inside the same realm, so the MV
// closure walk can stop at realm boundaries.
type AuthzResourceUsecase interface {
	repository.Getter[domain.AuthzResource]
	repository.Lister[domain.AuthzResource]
	repository.Patcher[domain.AuthzResource]
	repository.Deleter
	Create(ctx context.Context, r domain.AuthzResource) (domain.AuthzResource, error)
}

type authzResourceUsecase struct {
	usecase.Getter[domain.AuthzResource]
	usecase.Lister[domain.AuthzResource]
	repository.Patcher[domain.AuthzResource]
	repository.Deleter

	repo AuthzResourceRepository
}

func NewAuthzResourceUsecase(repo AuthzResourceRepository) AuthzResourceUsecase {
	return &authzResourceUsecase{
		Getter:  usecase.NewGetter(repo, domain.ResourceTypeAuthzResource),
		Lister:  usecase.NewLister(repo),
		Patcher: repo,
		Deleter: usecase.NewDeleter(repo),
		repo:    repo,
	}
}

func (uc *authzResourceUsecase) Create(ctx context.Context, r domain.AuthzResource) (domain.AuthzResource, error) {
	if err := validateAuthzResource(r); err != nil {
		return nil, err
	}
	if r.ParentID() != "" {
		parent, err := uc.repo.Get(ctx, byID(r.ParentID()))
		if err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return nil, apierrors.InvalidArgument("parent resource not found")
			}
			return nil, err
		}
		if parent.RealmID() != r.RealmID() {
			return nil, apierrors.InvalidArgument("parent resource belongs to a different realm")
		}
	}
	return uc.repo.Create(ctx, r)
}

func validateAuthzResource(r domain.AuthzResource) error {
	if r.RealmID() == "" {
		return apierrors.InvalidArgument("realm_id is required")
	}
	if r.ResourceType() == "" {
		return apierrors.InvalidArgument("type is required")
	}
	if !r.Visibility().Valid() {
		return apierrors.InvalidArgument("invalid visibility")
	}
	return nil
}
