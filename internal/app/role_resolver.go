package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence"
)

// RoleEffectivePermissionRepository persists the resolver's output cache — the
// fully folded, inheritance-expanded permission set per role that the
// projection joins.
type RoleEffectivePermissionRepository interface {
	DeleteAll(ctx context.Context) error
	CreateMany(ctx context.Context, roleID string, permissionIDs []string) error
}

// RoleResolver recomputes role_effective_permission from direct grants,
// composition, and inheritance. It runs before a projection refresh so the
// materialised view reflects the latest composed sets.
type RoleResolver interface {
	Resolve(ctx context.Context) error
}

type roleResolver struct {
	links        RolePermissionRepository
	compositions RoleCompositionRepository
	inheritance  PermissionInheritanceRepository
	effective    RoleEffectivePermissionRepository
	tx           persistence.Transactioner
}

func NewRoleResolver(
	links RolePermissionRepository,
	compositions RoleCompositionRepository,
	inheritance PermissionInheritanceRepository,
	effective RoleEffectivePermissionRepository,
	tx persistence.Transactioner,
) RoleResolver {
	return &roleResolver{
		links:        links,
		compositions: compositions,
		inheritance:  inheritance,
		effective:    effective,
		tx:           tx,
	}
}

func (r *roleResolver) Resolve(ctx context.Context) error {
	direct, err := r.links.ListAll(ctx)
	if err != nil {
		return err
	}
	comps, err := r.compositions.ListAll(ctx)
	if err != nil {
		return err
	}
	implied, err := r.inheritance.ListAllEdges(ctx)
	if err != nil {
		return err
	}

	resolved := ComposeEffectivePermissions(direct, comps, implied)

	return r.tx.Exec(ctx, func(ctx context.Context) error {
		if err := r.effective.DeleteAll(ctx); err != nil {
			return err
		}
		for roleID, perms := range resolved {
			if err := r.effective.CreateMany(ctx, roleID, perms); err != nil {
				return err
			}
		}
		return nil
	})
}
