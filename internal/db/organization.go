package db

import (
	"context"
	"encoding/json"
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

var organizationFieldMapping = map[string]string{
	fields.ID:         "id",
	fields.RealmID:    "realm_id",
	fields.ResourceID: "resource_id",
	fields.Name:       "name",
	fields.Slug:       "slug",
	fields.Status:     "status",
}

type organizationEntity struct {
	postgres.Model

	ERealmID    string  `gorm:"column:realm_id;type:uuid"`
	EResourceID string  `gorm:"column:resource_id;type:uuid"`
	EOwnerID    *string `gorm:"column:owner_account_id;type:uuid"`
	EName       string  `gorm:"column:name"`
	ESlug       string  `gorm:"column:slug"`
	EStatus     string  `gorm:"column:status"`
	ESettings   []byte  `gorm:"column:settings;type:jsonb"`
}

func (e *organizationEntity) TableName() string   { return "aegis.organization" }
func (e *organizationEntity) Type() resource.Type { return domain.ResourceTypeOrganization }

func (e *organizationEntity) Realm() resource.Identifier {
	return resource.NewIdentifier(e.ERealmID, domain.ResourceTypeRealm)
}

func (e *organizationEntity) AnchorResource() resource.Identifier {
	if e.EResourceID == "" {
		return nil
	}
	return resource.NewIdentifier(e.EResourceID, domain.ResourceTypeAuthzResource)
}

func (e *organizationEntity) Owner() resource.Identifier {
	if e.EOwnerID == nil || *e.EOwnerID == "" {
		return nil
	}
	return resource.NewIdentifier(*e.EOwnerID, domain.ResourceTypeAccount)
}

func (e *organizationEntity) Name() string             { return e.EName }
func (e *organizationEntity) Slug() string             { return e.ESlug }
func (e *organizationEntity) Status() domain.OrgStatus { return domain.OrgStatus(e.EStatus) }

func (e *organizationEntity) Settings() map[string]any {
	if len(e.ESettings) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(e.ESettings, &m); err != nil {
		return nil
	}
	return m
}

func organizationToEntity(o domain.Organization) *organizationEntity {
	e := &organizationEntity{
		Model:    postgres.ModelFromResource(o),
		ERealmID: o.Realm().ID(),
		EName:    o.Name(),
		ESlug:    o.Slug(),
		EStatus:  string(o.Status()),
	}
	if r := o.AnchorResource(); r != nil {
		e.EResourceID = r.ID()
	}
	if owner := o.Owner(); owner != nil {
		e.EOwnerID = nilIfEmpty(owner.ID())
	}
	if s := o.Settings(); s != nil {
		if b, err := json.Marshal(s); err == nil {
			e.ESettings = b
		}
	}
	return e
}

type organizationRepo struct {
	*postgres.Repo
}

func NewOrganizationRepository(db *gormdb.DBClient) (*organizationRepo, error) {
	r, err := postgres.NewRepo(db, organizationFieldMapping)
	if err != nil {
		return nil, err
	}
	return &organizationRepo{Repo: r}, nil
}

func (r *organizationRepo) Create(ctx context.Context, o domain.Organization) (domain.Organization, error) {
	e := organizationToEntity(o)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("organization", o.Slug())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *organizationRepo) Get(ctx context.Context, opts ...search.Option) (domain.Organization, error) {
	s := search.New(opts...)
	var e organizationEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("organization", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *organizationRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Organization], error) {
	s := search.New(opts...)
	var found []*organizationEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(organizationEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *organizationEntity) domain.Organization { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *organizationRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.Organization, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &organizationEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*organizationEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *organizationEntity) domain.Organization { return e }), nil
}

func (r *organizationRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&organizationEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&organizationEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// GetBySlug fetches an organization by its realm-unique slug.
func (r *organizationRepo) GetBySlug(ctx context.Context, realmID, slug string) (domain.Organization, error) {
	return r.Get(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Slug, slug),
	))
}
