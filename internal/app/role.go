package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// RoleRepository persists roles via kit generics.
type RoleRepository interface {
	repository.Creator[domain.Role]
	repository.Getter[domain.Role]
	repository.Lister[domain.Role]
	repository.Patcher[domain.Role]
	repository.Deleter
}

// RolePermissionRepository persists the role↔permission junction. The
// "atomic overwrite" of a role's permission set is composed at the usecase
// layer: DeleteByRole + CreateMany inside one Transactioner.Exec.
type RolePermissionRepository interface {
	DeleteByRole(ctx context.Context, roleID string) error
	CreateMany(ctx context.Context, roleID string, permissionIDs []string) error
	ListPermissionIDs(ctx context.Context, roleID string) ([]string, error)
	ListAll(ctx context.Context) (map[string][]string, error)
}

// RoleCompositionRepository persists composite-role definitions: ordered
// component roles folded with set operators. Overwrite is composed at the
// usecase layer like the permission junction.
type RoleCompositionRepository interface {
	DeleteByRole(ctx context.Context, roleID string) error
	CreateMany(ctx context.Context, roleID string, components []domain.RoleComponent) error
	ListComponents(ctx context.Context, roleID string) ([]domain.RoleComponent, error)
	ListAll(ctx context.Context) (map[string][]domain.RoleComponent, error)
}

// RoleUsecase is the admin surface for roles. Create validates the input,
// persists the role, then attaches the requested permissions atomically so a
// half-failed create can't leave an empty role.
type RoleUsecase interface {
	repository.Getter[domain.Role]
	repository.Lister[domain.Role]
	repository.Patcher[domain.Role]
	repository.Deleter
	Create(ctx context.Context, role domain.Role, permissionIDs []string) (domain.Role, error)
	SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error
	ListPermissions(ctx context.Context, roleID string) ([]string, error)
	SetComposition(ctx context.Context, roleID string, components []domain.RoleComponent) error
	ListComposition(ctx context.Context, roleID string) ([]domain.RoleComponent, error)
}

type roleUsecase struct {
	usecase.Getter[domain.Role]
	usecase.Lister[domain.Role]
	repository.Patcher[domain.Role]
	repository.Deleter

	roles        RoleRepository
	permissions  PermissionRepository
	links        RolePermissionRepository
	compositions RoleCompositionRepository
	tx           persistence.Transactioner
}

func NewRoleUsecase(
	roles RoleRepository,
	permissions PermissionRepository,
	links RolePermissionRepository,
	compositions RoleCompositionRepository,
	tx persistence.Transactioner,
) RoleUsecase {
	return &roleUsecase{
		Getter:       usecase.NewGetter(roles, domain.ResourceTypeRole),
		Lister:       usecase.NewLister(roles),
		Patcher:      roles,
		Deleter:      usecase.NewDeleter(roles),
		roles:        roles,
		permissions:  permissions,
		links:        links,
		compositions: compositions,
		tx:           tx,
	}
}

func (uc *roleUsecase) Create(ctx context.Context, role domain.Role, permissionIDs []string) (domain.Role, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}

	var out domain.Role
	err := uc.tx.Exec(ctx, func(ctx context.Context) error {
		created, err := uc.roles.Create(ctx, role)
		if err != nil {
			return err
		}
		if err := uc.attachPermissions(ctx, created.ID(), role.ResourceType(), permissionIDs); err != nil {
			return err
		}
		out = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (uc *roleUsecase) SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	role, err := uc.roles.Get(ctx, byID(roleID))
	if err != nil {
		return err
	}
	return uc.attachPermissions(ctx, roleID, role.ResourceType(), permissionIDs)
}

func (uc *roleUsecase) ListPermissions(ctx context.Context, roleID string) ([]string, error) {
	if _, err := uc.roles.Get(ctx, byID(roleID)); err != nil {
		return nil, err
	}
	return uc.links.ListPermissionIDs(ctx, roleID)
}

// attachPermissions verifies every requested permission exists and overwrites
// the role's permission set atomically (DELETE-then-INSERT inside the
// Transactioner). Empty permissionIDs clears the role's set.
func (uc *roleUsecase) attachPermissions(ctx context.Context, roleID, roleResourceType string, permissionIDs []string) error {
	for _, pid := range permissionIDs {
		p, err := uc.permissions.Get(ctx, byID(pid))
		if err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return apierrors.InvalidArgument("unknown permission")
			}
			return err
		}
		if p.ResourceType() != roleResourceType {
			return apierrors.InvalidArgument("permission resource_type does not match the role's resource_type")
		}
	}
	return uc.tx.Exec(ctx, func(ctx context.Context) error {
		if err := uc.links.DeleteByRole(ctx, roleID); err != nil {
			return err
		}
		return uc.links.CreateMany(ctx, roleID, permissionIDs)
	})
}

// SetComposition overwrites a role's composition. Every component must exist,
// share the composite's realm and resource_type, and differ from it — so a
// composite can only fold roles that grant the same kind of permission.
func (uc *roleUsecase) SetComposition(ctx context.Context, roleID string, components []domain.RoleComponent) error {
	role, err := uc.roles.Get(ctx, byID(roleID))
	if err != nil {
		return err
	}
	for _, c := range components {
		if !c.Operator.Valid() {
			return apierrors.InvalidArgument("invalid composition operator")
		}
		if c.ComponentRoleID == roleID {
			return apierrors.InvalidArgument("a role cannot compose itself")
		}
		component, err := uc.roles.Get(ctx, byID(c.ComponentRoleID))
		if err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return apierrors.InvalidArgument("unknown component role")
			}
			return err
		}
		if component.RealmID() != role.RealmID() || component.ResourceType() != role.ResourceType() {
			return apierrors.InvalidArgument("component role must share the realm and resource_type")
		}
	}
	return uc.tx.Exec(ctx, func(ctx context.Context) error {
		if err := uc.compositions.DeleteByRole(ctx, roleID); err != nil {
			return err
		}
		return uc.compositions.CreateMany(ctx, roleID, components)
	})
}

func (uc *roleUsecase) ListComposition(ctx context.Context, roleID string) ([]domain.RoleComponent, error) {
	if _, err := uc.roles.Get(ctx, byID(roleID)); err != nil {
		return nil, err
	}
	return uc.compositions.ListComponents(ctx, roleID)
}

func validateRole(r domain.Role) error {
	if r.RealmID() == "" {
		return apierrors.InvalidArgument("realm_id is required")
	}
	if r.Name() == "" {
		return apierrors.InvalidArgument("name is required")
	}
	if r.ResourceType() == "" {
		return apierrors.InvalidArgument("resource_type is required")
	}
	if !r.Kind().Valid() {
		return apierrors.InvalidArgument("invalid role kind")
	}
	return nil
}
