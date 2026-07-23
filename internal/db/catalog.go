package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
)

// catalogRepo holds the reconciliation primitives the catalog usecase needs
// beyond the generic permission/role repos: the applied-document record,
// managed_by stamping and listing, and the prune-safety counts.
type catalogRepo struct {
	db *gormdb.DBClient
}

func NewCatalogRepository(db *gormdb.DBClient) *catalogRepo {
	return &catalogRepo{db: db}
}

// catalogLockKey identifies the advisory lock serializing catalog
// reconciliation ("CTLG" in ASCII).
const catalogLockKey = 0x43544C47

// Lock takes the reconcile advisory lock for the duration of the surrounding
// transaction (pg_advisory_xact_lock blocks until granted and releases at
// commit/rollback). Callers MUST be inside a Transactioner.Exec.
func (r *catalogRepo) Lock(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec(`SELECT pg_advisory_xact_lock(?)`, catalogLockKey).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Upsert stores the applied document, bumping the revision on change.
func (r *catalogRepo) Upsert(ctx context.Context, resourceType string, document []byte) error {
	err := r.db.WithContext(ctx).Exec(
		`INSERT INTO aegis.catalog (resource_type, document)
		 VALUES (?, ?::jsonb)
		 ON CONFLICT (resource_type) DO UPDATE
		 SET document = EXCLUDED.document,
		     revision = aegis.catalog.revision + 1,
		     applied_at = NOW()`,
		resourceType, string(document)).Error
	if err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// StampPermission marks a permission row as managed by the catalog and keeps
// its description in sync with the document.
func (r *catalogRepo) StampPermission(ctx context.Context, permissionID, catalog, description string) error {
	err := r.db.WithContext(ctx).Exec(
		`UPDATE aegis.permission SET managed_by = ?, description = ? WHERE id = ?`,
		catalog, description, permissionID).Error
	if err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// StampRole marks a role row as managed by the catalog and keeps its
// description in sync with the document.
func (r *catalogRepo) StampRole(ctx context.Context, roleID, catalog, description string) error {
	err := r.db.WithContext(ctx).Exec(
		`UPDATE aegis.role SET managed_by = ?, description = ? WHERE id = ?`,
		catalog, description, roleID).Error
	if err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// ManagedPermissionIDs lists the permission ids the catalog currently owns.
func (r *catalogRepo) ManagedPermissionIDs(ctx context.Context, catalog string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM aegis.permission WHERE managed_by = ? ORDER BY id`, catalog).Scan(&ids).Error
	if err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return ids, nil
}

// ManagedRoleIDs lists the role ids the catalog currently owns.
func (r *catalogRepo) ManagedRoleIDs(ctx context.Context, catalog string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM aegis.role WHERE managed_by = ? AND deleted_at IS NULL ORDER BY id`, catalog).Scan(&ids).Error
	if err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return ids, nil
}

// PermissionManager reports who manages an existing permission ("" when
// unmanaged), so a catalog can adopt legacy rows but never steal another
// catalog's.
func (r *catalogRepo) PermissionManager(ctx context.Context, permissionID string) (string, error) {
	var managedBy *string
	err := r.db.WithContext(ctx).Raw(
		`SELECT managed_by FROM aegis.permission WHERE id = ?`, permissionID).Scan(&managedBy).Error
	if err != nil {
		return "", postgres.NewErrUnknown(err)
	}
	if managedBy == nil {
		return "", nil
	}
	return *managedBy, nil
}

// RoleManager reports who manages an existing role ("" when unmanaged).
func (r *catalogRepo) RoleManager(ctx context.Context, roleID string) (string, error) {
	var managedBy *string
	err := r.db.WithContext(ctx).Raw(
		`SELECT managed_by FROM aegis.role WHERE id = ? AND deleted_at IS NULL`, roleID).Scan(&managedBy).Error
	if err != nil {
		return "", postgres.NewErrUnknown(err)
	}
	if managedBy == nil {
		return "", nil
	}
	return *managedBy, nil
}

// CountBindingsByRole reports how many live grants reference the role — the
// prune-safety gate for removing a role from a catalog.
func (r *catalogRepo) CountBindingsByRole(ctx context.Context, roleID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM aegis.acl WHERE role_id = ? AND deleted_at IS NULL`, roleID).Scan(&n).Error
	if err != nil {
		return 0, postgres.NewErrUnknown(err)
	}
	return n, nil
}

// CountForeignPermissionLinks reports how many roles OUTSIDE the given set
// still attach the permission — the prune-safety gate for removing a
// permission (role_permission cascades on permission delete, which would
// silently strip it from custom roles).
func (r *catalogRepo) CountForeignPermissionLinks(ctx context.Context, permissionID string, ownRoleIDs []string) (int64, error) {
	var n int64
	q := `SELECT COUNT(*) FROM aegis.role_permission WHERE permission_id = ?`
	args := []any{permissionID}
	if len(ownRoleIDs) > 0 {
		q += ` AND role_id NOT IN (?)`
		args = append(args, ownRoleIDs)
	}
	if err := r.db.WithContext(ctx).Raw(q, args...).Scan(&n).Error; err != nil {
		return 0, postgres.NewErrUnknown(err)
	}
	return n, nil
}
