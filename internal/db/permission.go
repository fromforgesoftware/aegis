package db

import (
	"context"
	"errors"
	"time"

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

var permissionFieldMapping = map[string]string{
	fields.ID:           "id",
	fields.ResourceType: "resource_type",
	fields.Verb:         "verb",
}

// permissionEntity backs aegis.permission. The id is a TEXT slug (no UUID,
// no soft-delete) so we don't embed postgres.Model.
type permissionEntity struct {
	EID           string    `gorm:"column:id;primaryKey"`
	ECreatedAt    time.Time `gorm:"column:created_at;autoCreateTime:true"`
	EResourceType string    `gorm:"column:resource_type"`
	EVerb         string    `gorm:"column:verb"`
	EDescription  string    `gorm:"column:description"`
}

func (permissionEntity) TableName() string        { return "aegis.permission" }
func (e *permissionEntity) ID() string            { return e.EID }
func (e *permissionEntity) LID() string           { return "" }
func (e *permissionEntity) Type() resource.Type   { return domain.ResourceTypePermission }
func (e *permissionEntity) CreatedAt() time.Time  { return e.ECreatedAt }
func (e *permissionEntity) UpdatedAt() time.Time  { return e.ECreatedAt }
func (e *permissionEntity) DeletedAt() *time.Time { return nil }
func (e *permissionEntity) ResourceType() string  { return e.EResourceType }
func (e *permissionEntity) Verb() string          { return e.EVerb }
func (e *permissionEntity) Description() string   { return e.EDescription }

func permissionToEntity(p domain.Permission) *permissionEntity {
	return &permissionEntity{
		EID:           p.ID(),
		EResourceType: p.ResourceType(),
		EVerb:         p.Verb(),
		EDescription:  p.Description(),
	}
}

type permissionRepo struct {
	*postgres.Repo
}

func NewPermissionRepository(db *gormdb.DBClient) (*permissionRepo, error) {
	r, err := postgres.NewRepo(db, permissionFieldMapping)
	if err != nil {
		return nil, err
	}
	return &permissionRepo{Repo: r}, nil
}

func (r *permissionRepo) Create(ctx context.Context, p domain.Permission) (domain.Permission, error) {
	e := permissionToEntity(p)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("permission", p.ID())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *permissionRepo) Get(ctx context.Context, opts ...search.Option) (domain.Permission, error) {
	s := search.New(opts...)
	var e permissionEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("permission", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *permissionRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Permission], error) {
	s := search.New(opts...)
	var found []*permissionEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(permissionEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *permissionEntity) domain.Permission { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *permissionRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID),
		query.ValidFilter(fields.ID, filter.ValidateTyped[string])); err != nil {
		return err
	}
	if err := r.QueryApply(ctx, q).Delete(&permissionEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
