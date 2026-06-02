package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// effectiveAuthorizationRepo reads the effective_authorizations materialised
// view — the flattened binding closure. Each method is a single indexed
// lookup on the projection, never a recursive walk.
type effectiveAuthorizationRepo struct {
	db *gormdb.DBClient
}

func NewEffectiveAuthorizationRepository(db *gormdb.DBClient) *effectiveAuthorizationRepo {
	return &effectiveAuthorizationRepo{db: db}
}

func (r *effectiveAuthorizationRepo) Exists(ctx context.Context, accountID, resourceID, permissionID string) (bool, error) {
	var allowed bool
	err := r.db.WithContext(ctx).Raw(
		`SELECT EXISTS(
		     SELECT 1 FROM aegis.effective_authorizations
		     WHERE account_id = ? AND resource_id = ? AND permission_id = ?
		 )`,
		accountID, resourceID, permissionID).Scan(&allowed).Error
	if err != nil {
		return false, postgres.NewErrUnknown(err)
	}
	return allowed, nil
}

func (r *effectiveAuthorizationRepo) ListResourceIDs(ctx context.Context, accountID, permissionID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Raw(
		`SELECT resource_id FROM aegis.effective_authorizations
		 WHERE account_id = ? AND permission_id = ?
		 ORDER BY resource_id`,
		accountID, permissionID).Scan(&ids).Error
	if err != nil {
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
	var rows []domain.PermissionCheck
	err := r.db.WithContext(ctx).Raw(
		`SELECT resource_id, permission_id FROM aegis.effective_authorizations
		 WHERE account_id = ? AND resource_id IN (?) AND permission_id IN (?)`,
		accountID, resourceIDs, permissionIDs).Scan(&rows).Error
	if err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return rows, nil
}
