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

var authzResourceFieldMapping = map[string]string{
	fields.ID:             "id",
	fields.RealmID:        "realm_id",
	fields.ResourceType:   "type",
	fields.ParentID:       "parent_id",
	fields.OwnerAccountID: "owner_account_id",
	fields.Visibility:     "visibility",
}

type authzResourceEntity struct {
	postgres.Model

	ERealmID        string  `gorm:"column:realm_id;type:uuid"`
	EResourceType   string  `gorm:"column:type"`
	EOwnerAccountID *string `gorm:"column:owner_account_id;type:uuid"`
	EParentID       *string `gorm:"column:parent_id;type:uuid"`
	EInheritVia     string  `gorm:"column:inherit_via"`
	EVisibility     string  `gorm:"column:visibility"`
}

func (e *authzResourceEntity) TableName() string   { return "aegis.resource" }
func (e *authzResourceEntity) Type() resource.Type { return domain.ResourceTypeAuthzResource }
func (e *authzResourceEntity) RealmID() string     { return e.ERealmID }
func (e *authzResourceEntity) ResourceType() string {
	return e.EResourceType
}
func (e *authzResourceEntity) OwnerAccountID() string {
	if e.EOwnerAccountID == nil {
		return ""
	}
	return *e.EOwnerAccountID
}
func (e *authzResourceEntity) ParentID() string {
	if e.EParentID == nil {
		return ""
	}
	return *e.EParentID
}
func (e *authzResourceEntity) InheritVia() string {
	return e.EInheritVia
}
func (e *authzResourceEntity) Visibility() domain.Visibility {
	return domain.Visibility(e.EVisibility)
}

func authzResourceToEntity(r domain.AuthzResource) *authzResourceEntity {
	return &authzResourceEntity{
		Model:           postgres.ModelFromResource(r),
		ERealmID:        r.RealmID(),
		EResourceType:   r.ResourceType(),
		EOwnerAccountID: nilIfEmpty(r.OwnerAccountID()),
		EParentID:       nilIfEmpty(r.ParentID()),
		EInheritVia:     r.InheritVia(),
		EVisibility:     string(r.Visibility()),
	}
}

type authzResourceRepo struct {
	*postgres.Repo
}

func NewAuthzResourceRepository(db *gormdb.DBClient) (*authzResourceRepo, error) {
	r, err := postgres.NewRepo(db, authzResourceFieldMapping)
	if err != nil {
		return nil, err
	}
	return &authzResourceRepo{Repo: r}, nil
}

func (r *authzResourceRepo) Create(ctx context.Context, ar domain.AuthzResource) (domain.AuthzResource, error) {
	e := authzResourceToEntity(ar)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *authzResourceRepo) Get(ctx context.Context, opts ...search.Option) (domain.AuthzResource, error) {
	s := search.New(opts...)
	var e authzResourceEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("resource", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *authzResourceRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.AuthzResource], error) {
	s := search.New(opts...)
	var found []*authzResourceEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(authzResourceEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *authzResourceEntity) domain.AuthzResource { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *authzResourceRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.AuthzResource, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &authzResourceEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*authzResourceEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *authzResourceEntity) domain.AuthzResource { return e }), nil
}

func (r *authzResourceRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&authzResourceEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&authzResourceEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
