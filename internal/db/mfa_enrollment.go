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
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var mfaEnrollmentFieldMapping = map[string]string{
	fields.ID:        "id",
	fields.AccountID: "account_id",
	fields.Factor:    "factor",
}

type mfaEnrollmentEntity struct {
	postgres.Model

	EAccountID   string     `gorm:"column:account_id;type:uuid"`
	EFactor      string     `gorm:"column:factor"`
	ESecret      *string    `gorm:"column:secret"`
	EConfirmedAt *time.Time `gorm:"column:confirmed_at"`
}

func (e *mfaEnrollmentEntity) TableName() string        { return "aegis.mfa_enrollment" }
func (e *mfaEnrollmentEntity) Type() resource.Type      { return domain.ResourceTypeMFAEnrollment }
func (e *mfaEnrollmentEntity) AccountID() string        { return e.EAccountID }
func (e *mfaEnrollmentEntity) Factor() domain.MFAFactor { return domain.MFAFactor(e.EFactor) }
func (e *mfaEnrollmentEntity) ConfirmedAt() *time.Time  { return e.EConfirmedAt }
func (e *mfaEnrollmentEntity) Secret() string {
	if e.ESecret == nil {
		return ""
	}
	return *e.ESecret
}

type mfaEnrollmentRepo struct {
	*postgres.Repo
}

func NewMFAEnrollmentRepository(db *gormdb.DBClient) (*mfaEnrollmentRepo, error) {
	r, err := postgres.NewRepo(db, mfaEnrollmentFieldMapping)
	if err != nil {
		return nil, err
	}
	return &mfaEnrollmentRepo{Repo: r}, nil
}

// Upsert writes the enrollment, replacing any prior secret for the same
// (account, factor) — re-enrolling TOTP resets the secret and confirmation.
func (r *mfaEnrollmentRepo) Upsert(ctx context.Context, e domain.MFAEnrollment) (domain.MFAEnrollment, error) {
	row := &mfaEnrollmentEntity{
		Model:        postgres.ModelFromResource(e),
		EAccountID:   e.AccountID(),
		EFactor:      string(e.Factor()),
		ESecret:      nilIfEmpty(e.Secret()),
		EConfirmedAt: e.ConfirmedAt(),
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "factor"}},
		DoUpdates: clause.AssignmentColumns([]string{"secret", "confirmed_at", "updated_at"}),
	}).Create(row).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return row, nil
}

func (r *mfaEnrollmentRepo) GetByAccountFactor(ctx context.Context, accountID string, factor domain.MFAFactor) (domain.MFAEnrollment, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.AccountID, accountID),
		query.FilterBy(filter.OpEq, fields.Factor, string(factor)),
	)
	var e mfaEnrollmentEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("mfa enrollment", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

// Confirm marks a factor verified for the first time.
func (r *mfaEnrollmentRepo) Confirm(ctx context.Context, accountID string, factor domain.MFAFactor, at time.Time) error {
	if err := r.DB.WithContext(ctx).Exec(
		"UPDATE aegis.mfa_enrollment SET confirmed_at = ?, updated_at = NOW() WHERE account_id = ? AND factor = ?",
		at, accountID, string(factor)).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
