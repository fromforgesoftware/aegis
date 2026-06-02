package db

import (
	"context"
	"errors"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/slicesx"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var accountExternalIDFieldMapping = map[string]string{
	fields.AccountID:  "account_id",
	fields.Kind:       "kind",
	fields.ExternalID: "external_id",
}

type accountExternalIDEntity struct {
	EAccountID  string    `gorm:"column:account_id;type:uuid;primaryKey"`
	EKind       string    `gorm:"column:kind;primaryKey"`
	EExternalID string    `gorm:"column:external_id;primaryKey"`
	ECreatedAt  time.Time `gorm:"column:created_at;autoCreateTime:true"`
}

func (accountExternalIDEntity) TableName() string { return "aegis.account_external_id" }

func accountExternalIDToDomain(e *accountExternalIDEntity) domain.AccountExternalID {
	return domain.AccountExternalID{
		AccountID:  e.EAccountID,
		Kind:       domain.ExternalIDPKind(e.EKind),
		ExternalID: e.EExternalID,
		CreatedAt:  e.ECreatedAt,
	}
}

type accountExternalIDRepo struct {
	*postgres.Repo
}

func NewAccountExternalIDRepository(db *gormdb.DBClient) (*accountExternalIDRepo, error) {
	r, err := postgres.NewRepo(db, accountExternalIDFieldMapping)
	if err != nil {
		return nil, err
	}
	return &accountExternalIDRepo{Repo: r}, nil
}

func (r *accountExternalIDRepo) Create(ctx context.Context, link domain.AccountExternalID) error {
	e := &accountExternalIDEntity{
		EAccountID:  link.AccountID,
		EKind:       string(link.Kind),
		EExternalID: link.ExternalID,
	}
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return apierrors.AlreadyExists("account_external_id", link.ExternalID)
		}
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// GetByExternalID finds the link for an upstream identity. NotFound when
// the upstream identity hasn't been seen yet.
func (r *accountExternalIDRepo) GetByExternalID(ctx context.Context, kind domain.ExternalIDPKind, externalID string) (domain.AccountExternalID, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.Kind, string(kind)),
		query.FilterBy(filter.OpEq, fields.ExternalID, externalID),
	)
	var e accountExternalIDEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AccountExternalID{}, apierrors.NotFound("account_external_id", externalID)
		}
		return domain.AccountExternalID{}, postgres.NewErrUnknown(err)
	}
	return accountExternalIDToDomain(&e), nil
}

// ListByAccount returns every upstream link an Aegis account holds.
func (r *accountExternalIDRepo) ListByAccount(ctx context.Context, accountID string, opts ...search.Option) ([]domain.AccountExternalID, error) {
	base := []search.Option{search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.AccountID, accountID))}
	s := search.New(append(base, opts...)...)
	var found []*accountExternalIDEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *accountExternalIDEntity) domain.AccountExternalID {
		return accountExternalIDToDomain(e)
	}), nil
}
