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

var groupMemberFieldMapping = map[string]string{
	fields.GroupID:   "actor_set_id",
	fields.AccountID: "account_id",
}

type groupMemberEntity struct {
	EGroupID   string    `gorm:"column:actor_set_id;type:uuid;primaryKey"`
	EAccountID string    `gorm:"column:account_id;type:uuid;primaryKey"`
	ECreatedAt time.Time `gorm:"column:created_at;autoCreateTime:true"`
}

func (groupMemberEntity) TableName() string { return "aegis.actor_set_member" }

type groupMemberRepo struct {
	*postgres.Repo
}

func NewGroupMemberRepository(db *gormdb.DBClient) (*groupMemberRepo, error) {
	r, err := postgres.NewRepo(db, groupMemberFieldMapping)
	if err != nil {
		return nil, err
	}
	return &groupMemberRepo{Repo: r}, nil
}

// DeleteByGroup removes every membership row for groupID. Pair with
// CreateMany inside a usecase-level transaction for an atomic overwrite.
func (r *groupMemberRepo) DeleteByGroup(ctx context.Context, groupID string) error {
	q := query.New(query.FilterBy(filter.OpEq, fields.GroupID, groupID))
	if err := r.QueryApply(ctx, q).Delete(&groupMemberEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// CreateMany attaches accountIDs to groupID in a single batch insert.
// Caller already validated each account exists and shares the set's realm.
func (r *groupMemberRepo) CreateMany(ctx context.Context, groupID string, accountIDs []string) error {
	if len(accountIDs) == 0 {
		return nil
	}
	rows := make([]groupMemberEntity, 0, len(accountIDs))
	for _, aid := range accountIDs {
		rows = append(rows, groupMemberEntity{EGroupID: groupID, EAccountID: aid})
	}
	if err := r.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// ListAccountIDs returns the account ids in groupID, in stable order.
func (r *groupMemberRepo) ListAccountIDs(ctx context.Context, groupID string) ([]string, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.GroupID, groupID))
	var rows []groupMemberEntity
	if err := r.QueryApply(ctx, q).Order("account_id").Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.EAccountID)
	}
	return out, nil
}
