package db

import (
	"context"
	"errors"
	"time"

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

var bindingFieldMapping = map[string]string{
	fields.ID:          "id",
	fields.ResourceID:  "resource_id",
	fields.RoleID:      "role_id",
	fields.SubjectType: "subject_type",
	fields.SubjectID:   "subject_id",
}

type bindingEntity struct {
	postgres.Model

	EResourceID  string     `gorm:"column:resource_id;type:uuid"`
	ERoleID      string     `gorm:"column:role_id;type:uuid"`
	ESubjectType string     `gorm:"column:subject_type"`
	ESubjectID   string     `gorm:"column:subject_id;type:uuid"`
	EExpiresAt   *time.Time `gorm:"column:expires_at"`
}

func (e *bindingEntity) TableName() string   { return "aegis.acl" }
func (e *bindingEntity) Type() resource.Type { return domain.ResourceTypeBinding }
func (e *bindingEntity) ResourceID() string  { return e.EResourceID }
func (e *bindingEntity) RoleID() string      { return e.ERoleID }
func (e *bindingEntity) SubjectType() domain.SubjectType {
	return domain.SubjectType(e.ESubjectType)
}
func (e *bindingEntity) SubjectID() string     { return e.ESubjectID }
func (e *bindingEntity) ExpiresAt() *time.Time { return e.EExpiresAt }

func bindingToEntity(b domain.Binding) *bindingEntity {
	return &bindingEntity{
		Model:        postgres.ModelFromResource(b),
		EResourceID:  b.ResourceID(),
		ERoleID:      b.RoleID(),
		ESubjectType: string(b.SubjectType()),
		ESubjectID:   b.SubjectID(),
		EExpiresAt:   b.ExpiresAt(),
	}
}

type bindingRepo struct {
	*postgres.Repo
}

func NewBindingRepository(db *gormdb.DBClient) (*bindingRepo, error) {
	r, err := postgres.NewRepo(db, bindingFieldMapping)
	if err != nil {
		return nil, err
	}
	return &bindingRepo{Repo: r}, nil
}

func (r *bindingRepo) Create(ctx context.Context, b domain.Binding) (domain.Binding, error) {
	e := bindingToEntity(b)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("binding", b.SubjectID())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *bindingRepo) Get(ctx context.Context, opts ...search.Option) (domain.Binding, error) {
	s := search.New(opts...)
	var e bindingEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("binding", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *bindingRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Binding], error) {
	s := search.New(opts...)
	var found []*bindingEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(bindingEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *bindingEntity) domain.Binding { return e })
	return resource.NewListResponse(out, int(total)), nil
}

// DeleteExpired hard-deletes every binding whose expires_at has passed, and
// returns how many were removed. The sweeper calls this then refreshes.
func (r *bindingRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	tx := r.DB.WithContext(ctx).
		Exec("DELETE FROM aegis.acl WHERE expires_at IS NOT NULL AND expires_at <= ?", now)
	if tx.Error != nil {
		return 0, postgres.NewErrUnknown(tx.Error)
	}
	return tx.RowsAffected, nil
}

func (r *bindingRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&bindingEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&bindingEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
