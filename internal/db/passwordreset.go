package db

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/fields"
)

// passwordResetTokenEntity backs aegis.password_reset_token. Only the
// token hash is stored; the raw token lives only in the email.
type passwordResetTokenEntity struct {
	EID         string     `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	ECreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	EAccountID  string     `gorm:"column:account_id;type:uuid"`
	ETokenHash  string     `gorm:"column:token_hash"`
	EExpiresAt  time.Time  `gorm:"column:expires_at"`
	EConsumedAt *time.Time `gorm:"column:consumed_at"`
}

func (passwordResetTokenEntity) TableName() string { return "aegis.password_reset_token" }

type passwordResetTokenRepo struct {
	*postgres.Repo
}

func NewPasswordResetTokenRepository(db *gormdb.DBClient) (*passwordResetTokenRepo, error) {
	r, err := postgres.NewRepo(db, tokenFieldMapping)
	if err != nil {
		return nil, err
	}
	return &passwordResetTokenRepo{Repo: r}, nil
}

// Create inserts a new password reset token. Inserts stay raw GORM
// (QueryApply is for read/update/delete filtering).
func (r *passwordResetTokenRepo) Create(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error {
	e := &passwordResetTokenEntity{
		EAccountID: accountID,
		ETokenHash: tokenHash,
		EExpiresAt: expiresAt,
	}
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Consume atomically marks a valid (unconsumed, unexpired) token consumed
// and returns its account id. The single UPDATE ... WHERE consumed_at IS
// NULL avoids a check-then-use race; RETURNING reads the account id back
// in the same statement. NotFound when no valid token matches the hash.
func (r *passwordResetTokenRepo) Consume(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.TokenHash, tokenHash),
		query.FilterBy(filter.OpIsNull, fields.ConsumedAt, nil),
		query.FilterBy(filter.OpGT, fields.ExpiresAt, now),
	)

	e := &passwordResetTokenEntity{}
	res := r.QueryApply(ctx, q).
		Model(e).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "account_id"}}}).
		Update(r.FMapper()[fields.ConsumedAt], now)
	if res.Error != nil {
		return "", postgres.NewErrUnknown(res.Error)
	}
	if res.RowsAffected == 0 {
		return "", apierrors.NotFound("password reset token", "")
	}
	return e.EAccountID, nil
}
