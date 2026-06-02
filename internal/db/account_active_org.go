package db

import (
	"context"
	"errors"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/fields"
)

var accountActiveOrgFieldMapping = map[string]string{
	fields.AccountID: "account_id",
}

type accountActiveOrgEntity struct {
	EAccountID string    `gorm:"column:account_id;type:uuid;primaryKey"`
	EOrgID     string    `gorm:"column:organization_id;type:uuid"`
	EOrgRole   string    `gorm:"column:org_role"`
	EUpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (accountActiveOrgEntity) TableName() string { return "aegis.account_active_org" }

type accountActiveOrgRepo struct {
	*postgres.Repo
}

func NewAccountActiveOrgRepository(db *gormdb.DBClient) (*accountActiveOrgRepo, error) {
	r, err := postgres.NewRepo(db, accountActiveOrgFieldMapping)
	if err != nil {
		return nil, err
	}
	return &accountActiveOrgRepo{Repo: r}, nil
}

func (r *accountActiveOrgRepo) Get(ctx context.Context, accountID string) (orgID, orgRole string, found bool, err error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.AccountID, accountID))
	var e accountActiveOrgEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", false, nil
		}
		return "", "", false, postgres.NewErrUnknown(err)
	}
	return e.EOrgID, e.EOrgRole, true, nil
}

func (r *accountActiveOrgRepo) Set(ctx context.Context, accountID, orgID, orgRole string) error {
	e := &accountActiveOrgEntity{EAccountID: accountID, EOrgID: orgID, EOrgRole: orgRole}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"organization_id", "org_role", "updated_at"}),
	}).Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
