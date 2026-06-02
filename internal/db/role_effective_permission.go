package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
)

type roleEffectivePermissionEntity struct {
	ERoleID       string `gorm:"column:role_id;type:uuid;primaryKey"`
	EPermissionID string `gorm:"column:permission_id;primaryKey"`
}

func (roleEffectivePermissionEntity) TableName() string { return "aegis.role_effective_permission" }

type roleEffectivePermissionRepo struct {
	*postgres.Repo
}

func NewRoleEffectivePermissionRepository(db *gormdb.DBClient) (*roleEffectivePermissionRepo, error) {
	r, err := postgres.NewRepo(db, map[string]string{})
	if err != nil {
		return nil, err
	}
	return &roleEffectivePermissionRepo{Repo: r}, nil
}

// DeleteAll wipes the cache; the resolver rebuilds it wholesale inside one
// transaction so a check never sees a half-resolved set.
func (r *roleEffectivePermissionRepo) DeleteAll(ctx context.Context) error {
	if err := r.DB.WithContext(ctx).Exec("DELETE FROM aegis.role_effective_permission").Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

func (r *roleEffectivePermissionRepo) CreateMany(ctx context.Context, roleID string, permissionIDs []string) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([]roleEffectivePermissionEntity, 0, len(permissionIDs))
	for _, pid := range permissionIDs {
		rows = append(rows, roleEffectivePermissionEntity{ERoleID: roleID, EPermissionID: pid})
	}
	if err := r.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
