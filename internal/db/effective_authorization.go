package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/google/uuid"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// effectiveAuthorizationRepo answers authorization reads from two sources
// UNIONed per query: the effective_authorizations materialised view (the
// flattened binding closure) and the LIVE resource table's owner_account_id —
// a resource's owner implicitly holds every permission of the resource's type,
// with no binding, no role, and no projection refresh. Reading the owner from
// the live row (not the MV) makes create-then-check work immediately and
// delete-then-check deny immediately. Each branch is a single indexed lookup.
//
// The owner branch compares uuid columns, so it only runs when the ids parse
// as uuids (a non-uuid id can't own anything; casting it would error the
// whole query instead of denying).
type effectiveAuthorizationRepo struct {
	db *gormdb.DBClient
}

func NewEffectiveAuthorizationRepository(db *gormdb.DBClient) *effectiveAuthorizationRepo {
	return &effectiveAuthorizationRepo{db: db}
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func (r *effectiveAuthorizationRepo) Exists(ctx context.Context, accountID, resourceID, permissionID string) (bool, error) {
	q := `SELECT EXISTS(
	          SELECT 1 FROM aegis.effective_authorizations
	          WHERE account_id = ? AND resource_id = ? AND permission_id = ?
	      )`
	args := []any{accountID, resourceID, permissionID}
	if isUUID(accountID) && isUUID(resourceID) {
		q = `SELECT EXISTS(
		         SELECT 1 FROM aegis.effective_authorizations
		         WHERE account_id = ? AND resource_id = ? AND permission_id = ?
		         UNION ALL
		         SELECT 1 FROM aegis.resource r
		         JOIN aegis.permission p ON p.resource_type = r.type
		         WHERE r.id = ?::uuid AND r.owner_account_id = ?::uuid
		           AND p.id = ? AND r.deleted_at IS NULL
		     )`
		args = append(args, resourceID, accountID, permissionID)
	}
	var allowed bool
	if err := r.db.WithContext(ctx).Raw(q, args...).Scan(&allowed).Error; err != nil {
		return false, postgres.NewErrUnknown(err)
	}
	return allowed, nil
}

func (r *effectiveAuthorizationRepo) ListResourceIDs(ctx context.Context, accountID, permissionID string) ([]string, error) {
	q := `SELECT resource_id FROM aegis.effective_authorizations
	      WHERE account_id = ? AND permission_id = ?
	      ORDER BY resource_id`
	args := []any{accountID, permissionID}
	if isUUID(accountID) {
		q = `SELECT resource_id::text FROM aegis.effective_authorizations
		     WHERE account_id = ? AND permission_id = ?
		     UNION
		     SELECT r.id::text FROM aegis.resource r
		     JOIN aegis.permission p ON p.resource_type = r.type
		     WHERE r.owner_account_id = ?::uuid AND p.id = ?
		       AND r.deleted_at IS NULL
		     ORDER BY 1`
		args = append(args, accountID, permissionID)
	}
	var ids []string
	if err := r.db.WithContext(ctx).Raw(q, args...).Scan(&ids).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return ids, nil
}

// AllowedPairs returns the subset of checks that hold for the account, fetched
// in one query; the usecase maps the requested checks against this set.
func (r *effectiveAuthorizationRepo) AllowedPairs(ctx context.Context, accountID string, checks []domain.PermissionCheck) ([]domain.PermissionCheck, error) {
	if len(checks) == 0 {
		return nil, nil
	}
	resourceIDs := make([]string, 0, len(checks))
	permissionIDs := make([]string, 0, len(checks))
	for _, c := range checks {
		resourceIDs = append(resourceIDs, c.ResourceID)
		permissionIDs = append(permissionIDs, c.PermissionID)
	}
	q := `SELECT resource_id::text, permission_id FROM aegis.effective_authorizations
	      WHERE account_id = ? AND resource_id IN (?) AND permission_id IN (?)`
	args := []any{accountID, resourceIDs, permissionIDs}
	if isUUID(accountID) {
		q += `
		      UNION
		      SELECT r.id::text, p.id FROM aegis.resource r
		      JOIN aegis.permission p ON p.resource_type = r.type
		      WHERE r.owner_account_id = ?::uuid AND r.id::text IN (?)
		        AND p.id IN (?) AND r.deleted_at IS NULL`
		args = append(args, accountID, resourceIDs, permissionIDs)
	}
	var rows []domain.PermissionCheck
	if err := r.db.WithContext(ctx).Raw(q, args...).Scan(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return rows, nil
}
