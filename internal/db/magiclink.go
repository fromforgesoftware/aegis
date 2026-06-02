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

// magicLinkTokenEntity backs aegis.magic_link_token. Only the token hash is
// stored; the raw token lives only in the email.
type magicLinkTokenEntity struct {
	EID         string     `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	ECreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	EAccountID  string     `gorm:"column:account_id;type:uuid"`
	ETokenHash  string     `gorm:"column:token_hash"`
	EExpiresAt  time.Time  `gorm:"column:expires_at"`
	EConsumedAt *time.Time `gorm:"column:consumed_at"`
}

func (magicLinkTokenEntity) TableName() string { return "aegis.magic_link_token" }

type magicLinkTokenRepo struct {
	*postgres.Repo
}

func NewMagicLinkTokenRepository(db *gormdb.DBClient) (*magicLinkTokenRepo, error) {
	r, err := postgres.NewRepo(db, tokenFieldMapping)
	if err != nil {
		return nil, err
	}
	return &magicLinkTokenRepo{Repo: r}, nil
}

func (r *magicLinkTokenRepo) Create(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error {
	e := &magicLinkTokenEntity{EAccountID: accountID, ETokenHash: tokenHash, EExpiresAt: expiresAt}
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Consume atomically marks a valid (unconsumed, unexpired) token consumed and
// returns its account id in one UPDATE ... RETURNING, avoiding a
// check-then-use race. NotFound when no valid token matches the hash.
func (r *magicLinkTokenRepo) Consume(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.TokenHash, tokenHash),
		query.FilterBy(filter.OpIsNull, fields.ConsumedAt, nil),
		query.FilterBy(filter.OpGT, fields.ExpiresAt, now),
	)

	e := &magicLinkTokenEntity{}
	res := r.QueryApply(ctx, q).
		Model(e).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "account_id"}}}).
		Update(r.FMapper()[fields.ConsumedAt], now)
	if res.Error != nil {
		return "", postgres.NewErrUnknown(res.Error)
	}
	if res.RowsAffected == 0 {
		return "", apierrors.NotFound("magic link token", "")
	}
	return e.EAccountID, nil
}
