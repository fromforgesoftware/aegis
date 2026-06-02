package db

import (
	"context"
	"errors"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/slicesx"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var quotaPolicyFieldMapping = map[string]string{
	fields.ID:           "id",
	fields.RealmID:      "realm_id",
	fields.ResourceType: "resource_type",
}

type quotaPolicyEntity struct {
	postgres.Model

	ERealmID      string `gorm:"column:realm_id;type:uuid"`
	EResourceType string `gorm:"column:resource_type"`
	EMaxCount     int    `gorm:"column:max_count"`
}

func (e *quotaPolicyEntity) TableName() string    { return "aegis.realm_quota_policy" }
func (e *quotaPolicyEntity) Type() resource.Type  { return domain.ResourceTypeQuotaPolicy }
func (e *quotaPolicyEntity) RealmID() string      { return e.ERealmID }
func (e *quotaPolicyEntity) ResourceType() string { return e.EResourceType }
func (e *quotaPolicyEntity) MaxCount() int        { return e.EMaxCount }

func quotaPolicyToEntity(p domain.QuotaPolicy) *quotaPolicyEntity {
	return &quotaPolicyEntity{
		Model:         postgres.ModelFromResource(p),
		ERealmID:      p.RealmID(),
		EResourceType: p.ResourceType(),
		EMaxCount:     p.MaxCount(),
	}
}

type quotaPolicyRepo struct {
	*postgres.Repo
}

func NewQuotaPolicyRepository(db *gormdb.DBClient) (*quotaPolicyRepo, error) {
	r, err := postgres.NewRepo(db, quotaPolicyFieldMapping)
	if err != nil {
		return nil, err
	}
	return &quotaPolicyRepo{Repo: r}, nil
}

func (r *quotaPolicyRepo) Create(ctx context.Context, p domain.QuotaPolicy) (domain.QuotaPolicy, error) {
	e := quotaPolicyToEntity(p)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("realm quota policy", p.ResourceType())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *quotaPolicyRepo) Get(ctx context.Context, opts ...search.Option) (domain.QuotaPolicy, error) {
	s := search.New(opts...)
	var e quotaPolicyEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("realm quota policy", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *quotaPolicyRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.QuotaPolicy], error) {
	s := search.New(opts...)
	var found []*quotaPolicyEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(quotaPolicyEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *quotaPolicyEntity) domain.QuotaPolicy { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *quotaPolicyRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&quotaPolicyEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&quotaPolicyEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// GetByRealmResourceType loads the policy for a realm + resource type, or a
// NotFound when the realm sets no quota for it.
func (r *quotaPolicyRepo) GetByRealmResourceType(ctx context.Context, realmID, resourceType string) (domain.QuotaPolicy, error) {
	return r.Get(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.ResourceType, resourceType),
	))
}
