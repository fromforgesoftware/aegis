package app

import (
	"context"
	"strings"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// PermissionRepository persists the permission catalog via kit generics.
type PermissionRepository interface {
	repository.Creator[domain.Permission]
	repository.Getter[domain.Permission]
	repository.Lister[domain.Permission]
	repository.Deleter
}

// PermissionInheritanceRepository persists the implication DAG (P implies Q).
// The atomic overwrite of a permission's implications is composed at the
// usecase layer: DeleteByPermission + CreateMany inside one Transactioner.Exec.
type PermissionInheritanceRepository interface {
	DeleteByPermission(ctx context.Context, permissionID string) error
	CreateMany(ctx context.Context, permissionID string, impliedIDs []string) error
	ListImpliedIDs(ctx context.Context, permissionID string) ([]string, error)
	ListAllEdges(ctx context.Context) (map[string][]string, error)
}

// PermissionUsecase is the admin surface for the permission catalog plus its
// inheritance edges. Create validates the slug ↔ (resource_type, verb)
// agreement; SetImplications validates each implied permission exists.
type PermissionUsecase interface {
	repository.Creator[domain.Permission]
	repository.Getter[domain.Permission]
	repository.Lister[domain.Permission]
	repository.Deleter
	SetImplications(ctx context.Context, permissionID string, impliedIDs []string) error
	ListImplications(ctx context.Context, permissionID string) ([]string, error)
}

type permissionUsecase struct {
	usecase.Getter[domain.Permission]
	usecase.Lister[domain.Permission]
	repository.Deleter

	repo        PermissionRepository
	inheritance PermissionInheritanceRepository
	tx          persistence.Transactioner
}

func NewPermissionUsecase(
	repo PermissionRepository,
	inheritance PermissionInheritanceRepository,
	tx persistence.Transactioner,
) PermissionUsecase {
	return &permissionUsecase{
		Getter:      usecase.NewGetter(repo, domain.ResourceTypePermission),
		Lister:      usecase.NewLister(repo),
		Deleter:     usecase.NewDeleter(repo),
		repo:        repo,
		inheritance: inheritance,
		tx:          tx,
	}
}

func (uc *permissionUsecase) Create(ctx context.Context, p domain.Permission) (domain.Permission, error) {
	if err := validatePermission(p); err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, p)
}

// SetImplications overwrites the permission's implication set. Every implied
// permission must exist and differ from the base; the base must exist too.
func (uc *permissionUsecase) SetImplications(ctx context.Context, permissionID string, impliedIDs []string) error {
	if _, err := uc.repo.Get(ctx, byID(permissionID)); err != nil {
		return err
	}
	for _, id := range impliedIDs {
		if id == permissionID {
			return apierrors.InvalidArgument("a permission cannot imply itself")
		}
		if _, err := uc.repo.Get(ctx, byID(id)); err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return apierrors.InvalidArgument("unknown implied permission")
			}
			return err
		}
	}
	return uc.tx.Exec(ctx, func(ctx context.Context) error {
		if err := uc.inheritance.DeleteByPermission(ctx, permissionID); err != nil {
			return err
		}
		return uc.inheritance.CreateMany(ctx, permissionID, impliedIDs)
	})
}

func (uc *permissionUsecase) ListImplications(ctx context.Context, permissionID string) ([]string, error) {
	if _, err := uc.repo.Get(ctx, byID(permissionID)); err != nil {
		return nil, err
	}
	return uc.inheritance.ListImpliedIDs(ctx, permissionID)
}

func validatePermission(p domain.Permission) error {
	if p.ID() == "" || p.ResourceType() == "" || p.Verb() == "" {
		return apierrors.InvalidArgument("id, resource_type and verb are required")
	}
	if want := p.ResourceType() + "." + p.Verb(); want != p.ID() {
		return apierrors.InvalidArgument("permission id must be resource_type.verb")
	}
	if strings.ContainsAny(p.ResourceType(), ". ") || strings.ContainsAny(p.Verb(), ". ") {
		return apierrors.InvalidArgument("resource_type and verb cannot contain . or whitespace")
	}
	return nil
}
