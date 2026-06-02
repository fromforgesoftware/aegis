package db

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/fields"
)

var rolePermissionFieldMapping = map[string]string{
	fields.RoleID:       "role_id",
	fields.PermissionID: "permission_id",
}

type rolePermissionEntity struct {
	ERoleID       string    `gorm:"column:role_id;type:uuid;primaryKey"`
	EPermissionID string    `gorm:"column:permission_id;primaryKey"`
	ECreatedAt    time.Time `gorm:"column:created_at;autoCreateTime:true"`
}

func (rolePermissionEntity) TableName() string { return "aegis.role_permission" }

type rolePermissionRepo struct {
	*postgres.Repo
}

func NewRolePermissionRepository(db *gormdb.DBClient) (*rolePermissionRepo, error) {
	r, err := postgres.NewRepo(db, rolePermissionFieldMapping)
	if err != nil {
		return nil, err
	}
	return &rolePermissionRepo{Repo: r}, nil
}

// DeleteByRole removes every junction row for roleID. Pair with CreateMany
// inside a usecase-level transaction for an atomic overwrite.
func (r *rolePermissionRepo) DeleteByRole(ctx context.Context, roleID string) error {
	q := query.New(query.FilterBy(filter.OpEq, fields.RoleID, roleID))
	if err := r.QueryApply(ctx, q).Delete(&rolePermissionEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// CreateMany attaches permissionIDs to roleID in a single batch insert.
// Caller already validated each permission against the role's resource_type.
func (r *rolePermissionRepo) CreateMany(ctx context.Context, roleID string, permissionIDs []string) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([]rolePermissionEntity, 0, len(permissionIDs))
	for _, pid := range permissionIDs {
		rows = append(rows, rolePermissionEntity{ERoleID: roleID, EPermissionID: pid})
	}
	if err := r.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// ListAll returns every direct grant as role → permission ids, for the
// resolver's base layer.
func (r *rolePermissionRepo) ListAll(ctx context.Context) (map[string][]string, error) {
	var rows []rolePermissionEntity
	if err := r.DB.WithContext(ctx).Order("role_id, permission_id").Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := map[string][]string{}
	for _, row := range rows {
		out[row.ERoleID] = append(out[row.ERoleID], row.EPermissionID)
	}
	return out, nil
}

// ListPermissionIDs returns the slugs attached to roleID, in stable order.
func (r *rolePermissionRepo) ListPermissionIDs(ctx context.Context, roleID string) ([]string, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.RoleID, roleID))
	var rows []rolePermissionEntity
	if err := r.QueryApply(ctx, q).Order("permission_id").Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.EPermissionID)
	}
	return out, nil
}
