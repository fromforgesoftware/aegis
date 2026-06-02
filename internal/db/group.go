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

var groupFieldMapping = map[string]string{
	fields.ID:      "id",
	fields.RealmID: "realm_id",
	fields.Name:    "name",
}

type groupEntity struct {
	postgres.Model

	ERealmID        string  `gorm:"column:realm_id;type:uuid"`
	EName           string  `gorm:"column:name"`
	EDescription    string  `gorm:"column:description"`
	EOrganizationID *string `gorm:"column:organization_id;type:uuid"`
}

func (e *groupEntity) TableName() string   { return "aegis.actor_set" }
func (e *groupEntity) Type() resource.Type { return domain.ResourceTypeGroup }
func (e *groupEntity) RealmID() string     { return e.ERealmID }
func (e *groupEntity) Name() string        { return e.EName }
func (e *groupEntity) Description() string { return e.EDescription }
func (e *groupEntity) Organization() resource.Identifier {
	if e.EOrganizationID == nil || *e.EOrganizationID == "" {
		return nil
	}
	return resource.NewIdentifier(*e.EOrganizationID, domain.ResourceTypeOrganization)
}

func groupToEntity(a domain.Group) *groupEntity {
	e := &groupEntity{
		Model:        postgres.ModelFromResource(a),
		ERealmID:     a.RealmID(),
		EName:        a.Name(),
		EDescription: a.Description(),
	}
	if org := a.Organization(); org != nil {
		e.EOrganizationID = nilIfEmpty(org.ID())
	}
	return e
}

type groupRepo struct {
	*postgres.Repo
}

func NewGroupRepository(db *gormdb.DBClient) (*groupRepo, error) {
	r, err := postgres.NewRepo(db, groupFieldMapping)
	if err != nil {
		return nil, err
	}
	return &groupRepo{Repo: r}, nil
}

func (r *groupRepo) Create(ctx context.Context, a domain.Group) (domain.Group, error) {
	e := groupToEntity(a)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("group", a.Name())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *groupRepo) Get(ctx context.Context, opts ...search.Option) (domain.Group, error) {
	s := search.New(opts...)
	var e groupEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("group", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *groupRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Group], error) {
	s := search.New(opts...)
	var found []*groupEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(groupEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *groupEntity) domain.Group { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *groupRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.Group, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &groupEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*groupEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *groupEntity) domain.Group { return e }), nil
}

func (r *groupRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&groupEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&groupEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
