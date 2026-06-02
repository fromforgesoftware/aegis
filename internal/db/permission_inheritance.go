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

var permissionInheritanceFieldMapping = map[string]string{
	fields.PermissionID: "permission_id",
}

type permissionInheritanceEntity struct {
	EPermissionID        string    `gorm:"column:permission_id;primaryKey"`
	EImpliedPermissionID string    `gorm:"column:implied_permission_id;primaryKey"`
	ECreatedAt           time.Time `gorm:"column:created_at;autoCreateTime:true"`
}

func (permissionInheritanceEntity) TableName() string { return "aegis.permission_inheritance" }

type permissionInheritanceRepo struct {
	*postgres.Repo
}

func NewPermissionInheritanceRepository(db *gormdb.DBClient) (*permissionInheritanceRepo, error) {
	r, err := postgres.NewRepo(db, permissionInheritanceFieldMapping)
	if err != nil {
		return nil, err
	}
	return &permissionInheritanceRepo{Repo: r}, nil
}

// DeleteByPermission removes every implication edge out of permissionID. Pair
// with CreateMany inside a usecase transaction for an atomic overwrite.
func (r *permissionInheritanceRepo) DeleteByPermission(ctx context.Context, permissionID string) error {
	q := query.New(query.FilterBy(filter.OpEq, fields.PermissionID, permissionID))
	if err := r.QueryApply(ctx, q).Delete(&permissionInheritanceEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// CreateMany attaches impliedIDs as implications of permissionID.
func (r *permissionInheritanceRepo) CreateMany(ctx context.Context, permissionID string, impliedIDs []string) error {
	if len(impliedIDs) == 0 {
		return nil
	}
	rows := make([]permissionInheritanceEntity, 0, len(impliedIDs))
	for _, id := range impliedIDs {
		rows = append(rows, permissionInheritanceEntity{EPermissionID: permissionID, EImpliedPermissionID: id})
	}
	if err := r.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// ListImpliedIDs returns the permissions directly implied by permissionID.
func (r *permissionInheritanceRepo) ListImpliedIDs(ctx context.Context, permissionID string) ([]string, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.PermissionID, permissionID))
	var rows []permissionInheritanceEntity
	if err := r.QueryApply(ctx, q).Order("implied_permission_id").Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.EImpliedPermissionID)
	}
	return out, nil
}

// ListAllEdges returns the whole implication DAG as permission → implied ids,
// for the resolver.
func (r *permissionInheritanceRepo) ListAllEdges(ctx context.Context) (map[string][]string, error) {
	var rows []permissionInheritanceEntity
	if err := r.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := map[string][]string{}
	for _, row := range rows {
		out[row.EPermissionID] = append(out[row.EPermissionID], row.EImpliedPermissionID)
	}
	return out, nil
}
