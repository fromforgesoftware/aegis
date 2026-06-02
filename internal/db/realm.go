package db

import (
	"context"
	"errors"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
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

var realmFieldMapping = map[string]string{
	fields.ID:   "id",
	fields.Name: "name",
}

type realmEntity struct {
	postgres.Model

	EName        string `gorm:"column:name"`
	EDisplayName string `gorm:"column:display_name"`
}

func (e *realmEntity) TableName() string   { return "aegis.realm" }
func (e *realmEntity) Type() resource.Type { return domain.ResourceTypeRealm }
func (e *realmEntity) Name() string        { return e.EName }
func (e *realmEntity) DisplayName() string { return e.EDisplayName }

func realmToEntity(r domain.Realm) *realmEntity {
	return &realmEntity{
		Model:        postgres.ModelFromResource(r),
		EName:        r.Name(),
		EDisplayName: r.DisplayName(),
	}
}

type realmRepo struct {
	*postgres.Repo
}

func NewRealmRepository(db *gormdb.DBClient) (*realmRepo, error) {
	r, err := postgres.NewRepo(db, realmFieldMapping)
	if err != nil {
		return nil, err
	}
	return &realmRepo{Repo: r}, nil
}

func (r *realmRepo) Create(ctx context.Context, realm domain.Realm) (domain.Realm, error) {
	e := realmToEntity(realm)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("realm", realm.Name())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *realmRepo) Get(ctx context.Context, opts ...search.Option) (domain.Realm, error) {
	s := search.New(opts...)
	var e realmEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("realm", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *realmRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Realm], error) {
	s := search.New(opts...)
	var found []*realmEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(realmEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *realmEntity) domain.Realm { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *realmRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.Realm, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &realmEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*realmEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *realmEntity) domain.Realm { return e }), nil
}

func (r *realmRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&realmEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&realmEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
