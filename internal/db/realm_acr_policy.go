package db

import (
	"context"
	"errors"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var realmACRPolicyFieldMapping = map[string]string{
	fields.ID:      "id",
	fields.RealmID: "realm_id",
}

type realmACRPolicyEntity struct {
	postgres.Model

	ERealmID     string `gorm:"column:realm_id;type:uuid"`
	EMFARequired bool   `gorm:"column:mfa_required"`
	ERequiredACR string `gorm:"column:required_acr"`
}

func (e *realmACRPolicyEntity) TableName() string   { return "aegis.realm_acr_policy" }
func (e *realmACRPolicyEntity) Type() resource.Type { return domain.ResourceTypeRealmACRPolicy }
func (e *realmACRPolicyEntity) RealmID() string     { return e.ERealmID }
func (e *realmACRPolicyEntity) MFARequired() bool   { return e.EMFARequired }
func (e *realmACRPolicyEntity) RequiredACR() string { return e.ERequiredACR }

type realmACRPolicyRepo struct {
	*postgres.Repo
}

func NewRealmACRPolicyRepository(db *gormdb.DBClient) (*realmACRPolicyRepo, error) {
	r, err := postgres.NewRepo(db, realmACRPolicyFieldMapping)
	if err != nil {
		return nil, err
	}
	return &realmACRPolicyRepo{Repo: r}, nil
}

// Upsert sets the realm's policy, replacing any existing one (one per realm).
func (r *realmACRPolicyRepo) Upsert(ctx context.Context, p domain.RealmACRPolicy) (domain.RealmACRPolicy, error) {
	row := &realmACRPolicyEntity{
		Model:        postgres.ModelFromResource(p),
		ERealmID:     p.RealmID(),
		EMFARequired: p.MFARequired(),
		ERequiredACR: p.RequiredACR(),
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "realm_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"mfa_required", "required_acr", "updated_at"}),
	}).Create(row).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return row, nil
}

func (r *realmACRPolicyRepo) GetByRealm(ctx context.Context, realmID string) (domain.RealmACRPolicy, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.RealmID, realmID))
	var e realmACRPolicyEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("realm acr policy", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *realmACRPolicyRepo) Get(ctx context.Context, opts ...search.Option) (domain.RealmACRPolicy, error) {
	s := search.New(opts...)
	var e realmACRPolicyEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("realm acr policy", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}
