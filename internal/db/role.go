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

var roleFieldMapping = map[string]string{
	fields.ID:           "id",
	fields.RealmID:      "realm_id",
	fields.Name:         "name",
	fields.ResourceType: "resource_type",
	fields.Kind:         "kind",
}

type roleEntity struct {
	postgres.Model

	ERealmID      string `gorm:"column:realm_id;type:uuid"`
	EName         string `gorm:"column:name"`
	EResourceType string `gorm:"column:resource_type"`
	EDescription  string `gorm:"column:description"`
	EKind         string `gorm:"column:kind"`
}

func (e *roleEntity) TableName() string     { return "aegis.role" }
func (e *roleEntity) Type() resource.Type   { return domain.ResourceTypeRole }
func (e *roleEntity) RealmID() string       { return e.ERealmID }
func (e *roleEntity) Name() string          { return e.EName }
func (e *roleEntity) ResourceType() string  { return e.EResourceType }
func (e *roleEntity) Description() string   { return e.EDescription }
func (e *roleEntity) Kind() domain.RoleKind { return domain.RoleKind(e.EKind) }

func roleToEntity(r domain.Role) *roleEntity {
	return &roleEntity{
		Model:         postgres.ModelFromResource(r),
		ERealmID:      r.RealmID(),
		EName:         r.Name(),
		EResourceType: r.ResourceType(),
		EDescription:  r.Description(),
		EKind:         string(r.Kind()),
	}
}

type roleRepo struct {
	*postgres.Repo
}

func NewRoleRepository(db *gormdb.DBClient) (*roleRepo, error) {
	r, err := postgres.NewRepo(db, roleFieldMapping)
	if err != nil {
		return nil, err
	}
	return &roleRepo{Repo: r}, nil
}

func (r *roleRepo) Create(ctx context.Context, role domain.Role) (domain.Role, error) {
	e := roleToEntity(role)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("role", role.Name())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *roleRepo) Get(ctx context.Context, opts ...search.Option) (domain.Role, error) {
	s := search.New(opts...)
	var e roleEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("role", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *roleRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Role], error) {
	s := search.New(opts...)
	var found []*roleEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(roleEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *roleEntity) domain.Role { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *roleRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.Role, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &roleEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*roleEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *roleEntity) domain.Role { return e }), nil
}

func (r *roleRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&roleEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&roleEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
