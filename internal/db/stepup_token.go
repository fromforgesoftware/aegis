package db

import (
	"context"
	"errors"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

type stepUpTokenEntity struct {
	EID         string     `gorm:"column:id"`
	ECreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	EAccountID  string     `gorm:"column:account_id;type:uuid"`
	EFactor     string     `gorm:"column:factor"`
	EACR        string     `gorm:"column:acr"`
	EExpiresAt  time.Time  `gorm:"column:expires_at"`
	EConsumedAt *time.Time `gorm:"column:consumed_at"`
}

func (stepUpTokenEntity) TableName() string           { return "aegis.stepup_token" }
func (e *stepUpTokenEntity) ID() string               { return e.EID }
func (e *stepUpTokenEntity) LID() string              { return "" }
func (e *stepUpTokenEntity) Type() resource.Type      { return "stepUpTokens" }
func (e *stepUpTokenEntity) CreatedAt() time.Time     { return e.ECreatedAt }
func (e *stepUpTokenEntity) UpdatedAt() time.Time     { return e.ECreatedAt }
func (e *stepUpTokenEntity) DeletedAt() *time.Time    { return nil }
func (e *stepUpTokenEntity) AccountID() string        { return e.EAccountID }
func (e *stepUpTokenEntity) Factor() domain.MFAFactor { return domain.MFAFactor(e.EFactor) }
func (e *stepUpTokenEntity) ACR() string              { return e.EACR }
func (e *stepUpTokenEntity) ExpiresAt() time.Time     { return e.EExpiresAt }

type stepUpTokenRepo struct {
	*postgres.Repo
}

func NewStepUpTokenRepository(db *gormdb.DBClient) (*stepUpTokenRepo, error) {
	r, err := postgres.NewRepo(db, map[string]string{fields.ID: "id"})
	if err != nil {
		return nil, err
	}
	return &stepUpTokenRepo{Repo: r}, nil
}

func (r *stepUpTokenRepo) Create(ctx context.Context, t domain.StepUpToken) error {
	row := &stepUpTokenEntity{
		EID:        t.ID(),
		EAccountID: t.AccountID(),
		EFactor:    string(t.Factor()),
		EACR:       t.ACR(),
		EExpiresAt: t.ExpiresAt(),
	}
	if err := r.DB.WithContext(ctx).Create(row).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Verify returns the token if it exists, is unconsumed, and hasn't expired at
// `now`. A step-up token is reusable for sensitive operations within its TTL.
func (r *stepUpTokenRepo) Verify(ctx context.Context, id string, now time.Time) (domain.StepUpToken, error) {
	var e stepUpTokenEntity
	q := query.New(query.FilterBy(filter.OpEq, fields.ID, id))
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("step-up token", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	if e.EConsumedAt != nil || !e.EExpiresAt.After(now) {
		return nil, apierrors.New(apierrors.CodePreconditionFailed, apierrors.WithMessage("step-up token expired or consumed"))
	}
	return &e, nil
}
