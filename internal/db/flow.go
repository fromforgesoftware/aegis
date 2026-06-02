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

var flowFieldMapping = map[string]string{
	fields.ID:              "id",
	fields.RealmID:         "realm_id",
	fields.Type:            "type",
	fields.State:           "state",
	fields.ResultAccountID: "result_account_id",
	fields.ExpiresAt:       "expires_at",
}

// flowEntity backs aegis.flow and implements domain.Flow, so reads return
// it directly. postgres.Model supplies id + timestamps.
type flowEntity struct {
	postgres.Model

	ERealmID         string    `gorm:"column:realm_id;type:uuid"`
	EType            string    `gorm:"column:type"`
	EState           string    `gorm:"column:state"`
	EResultAccountID *string   `gorm:"column:result_account_id;type:uuid"`
	EExpiresAt       time.Time `gorm:"column:expires_at"`
}

func (e *flowEntity) TableName() string         { return "aegis.flow" }
func (e *flowEntity) Type() resource.Type       { return domain.ResourceTypeFlow }
func (e *flowEntity) RealmID() string           { return e.ERealmID }
func (e *flowEntity) FlowType() domain.FlowType { return domain.FlowType(e.EType) }
func (e *flowEntity) State() domain.FlowState   { return domain.FlowState(e.EState) }
func (e *flowEntity) ExpiresAt() time.Time      { return e.EExpiresAt }

func (e *flowEntity) ResultAccountID() string {
	if e.EResultAccountID == nil {
		return ""
	}
	return *e.EResultAccountID
}

func flowToEntity(f domain.Flow) *flowEntity {
	e := &flowEntity{
		Model:      postgres.ModelFromResource(f),
		ERealmID:   f.RealmID(),
		EType:      string(f.FlowType()),
		EState:     string(f.State()),
		EExpiresAt: f.ExpiresAt(),
	}
	if id := f.ResultAccountID(); id != "" {
		e.EResultAccountID = &id
	}
	return e
}

type flowRepo struct {
	*postgres.Repo
}

func NewFlowRepository(db *gormdb.DBClient) (*flowRepo, error) {
	r, err := postgres.NewRepo(db, flowFieldMapping)
	if err != nil {
		return nil, err
	}
	return &flowRepo{Repo: r}, nil
}

func (r *flowRepo) Create(ctx context.Context, f domain.Flow) (domain.Flow, error) {
	e := flowToEntity(f)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *flowRepo) Get(ctx context.Context, opts ...search.Option) (domain.Flow, error) {
	q := search.New(opts...).Query()
	if err := query.Validate(q,
		query.MandatoryFilters(fields.ID),
		query.ValidFilter(fields.ID, filter.ValidateTyped[string], filter.ValidateUUID),
	); err != nil {
		return nil, err
	}
	var e flowEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("flow", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *flowRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.Flow, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q,
		query.MandatoryFilters(fields.ID),
		query.ValidFilter(fields.ID, filter.ValidateTyped[string], filter.ValidateUUID),
	); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &flowEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*flowEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *flowEntity) domain.Flow { return e }), nil
}
