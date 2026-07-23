package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// CatalogRepository holds the reconciliation primitives beyond the generic
// permission/role repos: the reconcile lock, the applied-document record,
// managed_by stamping and listing, and the prune-safety counts.
type CatalogRepository interface {
	// Lock serializes catalog reconciliation across processes for the duration
	// of the surrounding transaction (advisory xact lock) — concurrent pods
	// applying at startup converge instead of interleaving.
	Lock(ctx context.Context) error
	Upsert(ctx context.Context, resourceType string, document []byte) error
	StampPermission(ctx context.Context, permissionID, catalog, description string) error
	StampRole(ctx context.Context, roleID, catalog, description string) error
	ManagedPermissionIDs(ctx context.Context, catalog string) ([]string, error)
	ManagedRoleIDs(ctx context.Context, catalog string) ([]string, error)
	PermissionManager(ctx context.Context, permissionID string) (string, error)
	RoleManager(ctx context.Context, roleID string) (string, error)
	CountBindingsByRole(ctx context.Context, roleID string) (int64, error)
	CountForeignPermissionLinks(ctx context.Context, permissionID string, ownRoleIDs []string) (int64, error)
}

// CatalogUsecase reconciles declarative authz catalogs — a resource type's
// permissions (implication tree) and SYSTEM roles (composition tree) — into
// the stored vocabulary. Apply converges add/update/remove in one transaction
// per document, then refreshes the projection once. It is provisioning-time
// machinery (config-mounted documents applied at startup), not a request-path
// API.
type CatalogUsecase interface {
	Apply(ctx context.Context, docs []domain.CatalogDocument) error
}

type catalogUsecase struct {
	permissions PermissionRepository
	inheritance PermissionInheritanceRepository
	roles       RoleRepository
	links       RolePermissionRepository
	comps       RoleCompositionRepository
	catalogs    CatalogRepository
	authz       AuthorizationUsecase
	auditor     Auditor
	tx          persistence.Transactioner
}

func NewCatalogUsecase(
	permissions PermissionRepository,
	inheritance PermissionInheritanceRepository,
	roles RoleRepository,
	links RolePermissionRepository,
	comps RoleCompositionRepository,
	catalogs CatalogRepository,
	authz AuthorizationUsecase,
	auditor Auditor,
	tx persistence.Transactioner,
) CatalogUsecase {
	return &catalogUsecase{
		permissions: permissions,
		inheritance: inheritance,
		roles:       roles,
		links:       links,
		comps:       comps,
		catalogs:    catalogs,
		authz:       authz,
		auditor:     auditor,
		tx:          tx,
	}
}

// Apply validates every document first (an invalid catalog applies nothing),
// converges each in its own transaction, then refreshes the projection once
// so re-resolved roles land in the closure.
func (uc *catalogUsecase) Apply(ctx context.Context, docs []domain.CatalogDocument) error {
	log := logger.New()
	seen := map[string]bool{}
	for _, d := range docs {
		if err := d.Validate(); err != nil {
			return apierrors.InvalidArgument(err.Error())
		}
		if seen[d.ResourceType] {
			return apierrors.InvalidArgument(fmt.Sprintf("catalog %q declared twice", d.ResourceType))
		}
		seen[d.ResourceType] = true
		// A force override should be a one-deploy affair; nag on every apply
		// so a leftover entry can't quietly disable prune safety forever.
		if len(d.Force) > 0 {
			log.Warn("catalog carries destructive force overrides — remove them after the prune lands",
				"catalog", d.ResourceType, "force", d.Force)
		}
	}
	for _, d := range docs {
		if err := uc.tx.Exec(ctx, func(txCtx context.Context) error {
			return uc.applyOne(txCtx, d)
		}); err != nil {
			return fmt.Errorf("apply catalog %q: %w", d.ResourceType, err)
		}
		uc.auditor.Record(ctx, "catalog.apply", "catalog", d.ResourceType, map[string]any{
			"permissions": len(d.Permissions),
			"roles":       len(d.Roles),
			"force":       d.Force,
		})
	}
	if len(docs) == 0 {
		return nil
	}
	return uc.authz.Refresh(ctx)
}

func (uc *catalogUsecase) applyOne(ctx context.Context, d domain.CatalogDocument) error {
	// Serialize appliers: with several replicas (or an overlapping rollout)
	// every pod reconciles at startup; the xact lock makes the second one
	// converge over the first's committed work instead of interleaving.
	if err := uc.catalogs.Lock(ctx); err != nil {
		return err
	}
	if err := uc.convergePermissions(ctx, d); err != nil {
		return err
	}
	if err := uc.convergeRoles(ctx, d); err != nil {
		return err
	}
	if err := uc.prune(ctx, d); err != nil {
		return err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return uc.catalogs.Upsert(ctx, d.ResourceType, raw)
}

// convergePermissions ensures every declared permission exists (adopting
// unmanaged legacy rows, refusing rows another catalog manages), stamps it
// (managed_by, description, the members grant), and overwrites its implication
// edges to match the document.
func (uc *catalogUsecase) convergePermissions(ctx context.Context, d domain.CatalogDocument) error {
	for _, verb := range catalogKeys(d.Permissions) {
		spec := d.Permissions[verb]
		id := d.PermissionID(verb)
		if _, err := uc.permissions.Get(ctx, byID(id)); err != nil {
			if !apierrors.Is(err, apierrors.CodeNotFound) {
				return err
			}
			// A lost create race (another applier of this catalog) is
			// convergence, not failure; the manager check below still rejects
			// rows owned by a DIFFERENT catalog.
			if _, err := uc.permissions.Create(ctx, domain.NewPermission(id, d.ResourceType, verb,
				domain.WithPermissionDescription(spec.Description))); err != nil && !apierrors.Is(err, apierrors.CodeAlreadyExists) {
				return err
			}
		}
		manager, err := uc.catalogs.PermissionManager(ctx, id)
		if err != nil {
			return err
		}
		if manager != "" && manager != d.ResourceType {
			return apierrors.InvalidArgument(fmt.Sprintf("permission %q is managed by catalog %q", id, manager))
		}
		if err := uc.catalogs.StampPermission(ctx, id, d.ResourceType, spec.Description); err != nil {
			return err
		}
		if err := uc.inheritance.DeleteByPermission(ctx, id); err != nil {
			return err
		}
		implied := qualifyAll(d, spec.Implies, d.PermissionID)
		if len(implied) > 0 {
			if err := uc.inheritance.CreateMany(ctx, id, implied); err != nil {
				return err
			}
		}
	}
	return nil
}

// convergeRoles ensures every declared SYSTEM role exists under its slug id
// (realm-less), stamps it, then overwrites its permission links and its
// composition to match the document. Roles are all created before any
// composition is written so component FKs always resolve.
func (uc *catalogUsecase) convergeRoles(ctx context.Context, d domain.CatalogDocument) error {
	names := catalogKeys(d.Roles)
	for _, name := range names {
		id := d.RoleID(name)
		if _, err := uc.roles.Get(ctx, byID(id)); err != nil {
			if !apierrors.Is(err, apierrors.CodeNotFound) {
				return err
			}
			// Lost create races converge; the manager check still rejects rows
			// owned by a DIFFERENT catalog.
			if _, err := uc.roles.Create(ctx, domain.NewRole("", name, d.ResourceType,
				domain.WithRoleID(id),
				domain.WithRoleKind(domain.RoleKindSystem),
				domain.WithRoleDescription(d.Roles[name].Description))); err != nil && !apierrors.Is(err, apierrors.CodeAlreadyExists) {
				return err
			}
		}
		manager, err := uc.catalogs.RoleManager(ctx, id)
		if err != nil {
			return err
		}
		if manager != "" && manager != d.ResourceType {
			return apierrors.InvalidArgument(fmt.Sprintf("role %q is managed by catalog %q", id, manager))
		}
		if err := uc.catalogs.StampRole(ctx, id, d.ResourceType, d.Roles[name].Description); err != nil {
			return err
		}
	}
	for _, name := range names {
		spec := d.Roles[name]
		id := d.RoleID(name)
		if err := uc.links.DeleteByRole(ctx, id); err != nil {
			return err
		}
		if perms := qualifyAll(d, spec.Permissions, d.PermissionID); len(perms) > 0 {
			if err := uc.links.CreateMany(ctx, id, perms); err != nil {
				return err
			}
		}
		if err := uc.comps.DeleteByRole(ctx, id); err != nil {
			return err
		}
		if len(spec.Composes) > 0 {
			components := make([]domain.RoleComponent, 0, len(spec.Composes))
			for i, ref := range qualifyAll(d, spec.Composes, d.RoleID) {
				components = append(components, domain.RoleComponent{
					ComponentRoleID: ref,
					Operator:        domain.CompositionUnion,
					Ordinal:         i,
				})
			}
			if err := uc.comps.CreateMany(ctx, id, components); err != nil {
				return err
			}
		}
	}
	return nil
}

// prune removes rows this catalog manages that left the document. Removing a
// role with live bindings, or a permission other roles still attach, is
// refused unless the document sets force — a slimmed document must not
// silently revoke access.
func (uc *catalogUsecase) prune(ctx context.Context, d domain.CatalogDocument) error {
	managedRoles, err := uc.catalogs.ManagedRoleIDs(ctx, d.ResourceType)
	if err != nil {
		return err
	}
	keepRoles := map[string]bool{}
	for name := range d.Roles {
		keepRoles[d.RoleID(name)] = true
	}
	for _, id := range managedRoles {
		if keepRoles[id] {
			continue
		}
		if !d.Forces(id) {
			n, err := uc.catalogs.CountBindingsByRole(ctx, id)
			if err != nil {
				return err
			}
			if n > 0 {
				return apierrors.InvalidArgument(fmt.Sprintf("cannot prune role %q: %d live bindings (add it to force to override)", id, n))
			}
		}
		if err := uc.roles.Delete(ctx, repository.DeleteTypeHard, byID(id)); err != nil {
			return err
		}
	}

	managedPerms, err := uc.catalogs.ManagedPermissionIDs(ctx, d.ResourceType)
	if err != nil {
		return err
	}
	keepPerms := map[string]bool{}
	for verb := range d.Permissions {
		keepPerms[d.PermissionID(verb)] = true
	}
	ownRoles := append(managedRoles[:len(managedRoles):len(managedRoles)], sortedRoleIDs(d)...)
	for _, id := range managedPerms {
		if keepPerms[id] {
			continue
		}
		if !d.Forces(id) {
			n, err := uc.catalogs.CountForeignPermissionLinks(ctx, id, ownRoles)
			if err != nil {
				return err
			}
			if n > 0 {
				return apierrors.InvalidArgument(fmt.Sprintf("cannot prune permission %q: attached to %d roles outside this catalog (add it to force to override)", id, n))
			}
		}
		if err := uc.permissions.Delete(ctx, repository.DeleteTypeHard, byID(id)); err != nil {
			return err
		}
	}
	return nil
}

func qualifyAll(d domain.CatalogDocument, refs []string, qualify func(string) string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, qualify(d.Normalize(r)))
	}
	return out
}

func sortedRoleIDs(d domain.CatalogDocument) []string {
	out := make([]string, 0, len(d.Roles))
	for name := range d.Roles {
		out = append(out, d.RoleID(name))
	}
	sort.Strings(out)
	return out
}

func catalogKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
