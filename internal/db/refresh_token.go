package db

import (
	"context"
	"errors"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var refreshTokenFieldMapping = map[string]string{
	fields.ID:        "id",
	fields.TokenHash: "token_hash",
	fields.SessionID: "session_id",
	fields.UsedAt:    "used_at",
}

type refreshTokenEntity struct {
	EID          string     `gorm:"column:id;primaryKey;default:uuid_generate_v4()"`
	ECreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	ESessionID   string     `gorm:"column:session_id;type:uuid"`
	EClientID    string     `gorm:"column:client_id"`
	ETokenHash   string     `gorm:"column:token_hash"`
	EScopes      []string   `gorm:"column:scopes;type:jsonb;serializer:json"`
	ERotatedFrom *string    `gorm:"column:rotated_from;type:uuid"`
	EUsedAt      *time.Time `gorm:"column:used_at"`
	EExpiresAt   time.Time  `gorm:"column:expires_at"`
}

func (refreshTokenEntity) TableName() string { return "aegis.refresh_token" }

func refreshTokenToDomain(e *refreshTokenEntity) domain.RefreshToken {
	var rotatedFrom string
	if e.ERotatedFrom != nil {
		rotatedFrom = *e.ERotatedFrom
	}
	return domain.RefreshToken{
		ID:          e.EID,
		SessionID:   e.ESessionID,
		ClientID:    e.EClientID,
		TokenHash:   e.ETokenHash,
		Scopes:      e.EScopes,
		RotatedFrom: rotatedFrom,
		UsedAt:      e.EUsedAt,
		ExpiresAt:   e.EExpiresAt,
	}
}

type refreshTokenRepo struct {
	*postgres.Repo
}

func NewRefreshTokenRepository(db *gormdb.DBClient) (*refreshTokenRepo, error) {
	r, err := postgres.NewRepo(db, refreshTokenFieldMapping)
	if err != nil {
		return nil, err
	}
	return &refreshTokenRepo{Repo: r}, nil
}

func (r *refreshTokenRepo) Create(ctx context.Context, t domain.RefreshToken) error {
	e := &refreshTokenEntity{
		ESessionID:   t.SessionID,
		EClientID:    t.ClientID,
		ETokenHash:   t.TokenHash,
		EScopes:      orEmptySlice(t.Scopes),
		ERotatedFrom: nilIfEmpty(t.RotatedFrom),
		EExpiresAt:   t.ExpiresAt,
	}
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

func (r *refreshTokenRepo) GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.TokenHash, hash))
	var e refreshTokenEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RefreshToken{}, apierrors.NotFound("refresh token", "")
		}
		return domain.RefreshToken{}, postgres.NewErrUnknown(err)
	}
	return refreshTokenToDomain(&e), nil
}

// MarkUsed atomically claims an unused token; a false return means a concurrent reuse already rotated it.
func (r *refreshTokenRepo) MarkUsed(ctx context.Context, id string, now time.Time) (bool, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.ID, id),
		query.FilterBy(filter.OpIsNull, fields.UsedAt, nil),
	)
	res := r.QueryApply(ctx, q).
		Model(&refreshTokenEntity{}).
		Update(r.FMapper()[fields.UsedAt], now)
	if res.Error != nil {
		return false, postgres.NewErrUnknown(res.Error)
	}
	return res.RowsAffected == 1, nil
}
