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

// tokenFieldMapping is shared by both single-use token repositories
// (verification + password reset): the two tables are structurally
// identical, so they map the same logical fields to the same columns.
var tokenFieldMapping = map[string]string{
	fields.AccountID:  "account_id",
	fields.TokenHash:  "token_hash",
	fields.ExpiresAt:  "expires_at",
	fields.ConsumedAt: "consumed_at",
}

// emailVerificationTokenEntity backs aegis.email_verification_token. Only
// the token hash is stored; the raw token lives only in the email.
type emailVerificationTokenEntity struct {
	EID         string     `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	ECreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	EAccountID  string     `gorm:"column:account_id;type:uuid"`
	ETokenHash  string     `gorm:"column:token_hash"`
	EExpiresAt  time.Time  `gorm:"column:expires_at"`
	EConsumedAt *time.Time `gorm:"column:consumed_at"`
}

func (emailVerificationTokenEntity) TableName() string { return "aegis.email_verification_token" }

type verificationTokenRepo struct {
	*postgres.Repo
}

func NewVerificationTokenRepository(db *gormdb.DBClient) (*verificationTokenRepo, error) {
	r, err := postgres.NewRepo(db, tokenFieldMapping)
	if err != nil {
		return nil, err
	}
	return &verificationTokenRepo{Repo: r}, nil
}

// Create inserts a new verification token. Inserts stay raw GORM
// (QueryApply is for read/update/delete filtering).
func (r *verificationTokenRepo) Create(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error {
	e := &emailVerificationTokenEntity{
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
// NULL avoids a check-then-use race (double consumption); RETURNING reads
// the account id back in the same statement. NotFound when no valid token
// matches the hash.
func (r *verificationTokenRepo) Consume(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.TokenHash, tokenHash),
		query.FilterBy(filter.OpIsNull, fields.ConsumedAt, nil),
		query.FilterBy(filter.OpGT, fields.ExpiresAt, now),
	)

	e := &emailVerificationTokenEntity{}
	res := r.QueryApply(ctx, q).
		Model(e).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "account_id"}}}).
		Update(r.FMapper()[fields.ConsumedAt], now)
	if res.Error != nil {
		return "", postgres.NewErrUnknown(res.Error)
	}
	if res.RowsAffected == 0 {
		return "", apierrors.NotFound("verification token", "")
	}
	return e.EAccountID, nil
}
