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
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var sessionFieldMapping = map[string]string{
	fields.ID:        "id",
	fields.RealmID:   "realm_id",
	fields.AccountID: "account_id",
	fields.RevokedAt: "revoked_at",
}

type sessionEntity struct {
	postgres.Model

	ERealmID   string     `gorm:"column:realm_id;type:uuid"`
	EAccountID string     `gorm:"column:account_id;type:uuid"`
	EExpiresAt time.Time  `gorm:"column:expires_at"`
	ERevokedAt *time.Time `gorm:"column:revoked_at"`
}

func (e *sessionEntity) TableName() string     { return "aegis.session" }
func (e *sessionEntity) Type() resource.Type   { return domain.ResourceTypeSession }
func (e *sessionEntity) RealmID() string       { return e.ERealmID }
func (e *sessionEntity) AccountID() string     { return e.EAccountID }
func (e *sessionEntity) ExpiresAt() time.Time  { return e.EExpiresAt }
func (e *sessionEntity) RevokedAt() *time.Time { return e.ERevokedAt }

func sessionToEntity(s domain.Session) *sessionEntity {
	return &sessionEntity{
		Model:      postgres.ModelFromResource(s),
		ERealmID:   s.RealmID(),
		EAccountID: s.AccountID(),
		EExpiresAt: s.ExpiresAt(),
		ERevokedAt: s.RevokedAt(),
	}
}

type sessionRepo struct {
	*postgres.Repo
}

func NewSessionRepository(db *gormdb.DBClient) (*sessionRepo, error) {
	r, err := postgres.NewRepo(db, sessionFieldMapping)
	if err != nil {
		return nil, err
	}
	return &sessionRepo{Repo: r}, nil
}

func (r *sessionRepo) Create(ctx context.Context, s domain.Session) (domain.Session, error) {
	e := sessionToEntity(s)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *sessionRepo) Get(ctx context.Context, opts ...search.Option) (domain.Session, error) {
	s := search.New(opts...)
	var e sessionEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("session", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

// Revoke stamps revoked_at on the session so SessionActive rejects it. Used
// to kill a refresh-token chain after a detected reuse.
func (r *sessionRepo) Revoke(ctx context.Context, sessionID string, now time.Time) error {
	q := query.New(query.FilterBy(filter.OpEq, fields.ID, sessionID))
	if err := r.PatchApply(ctx, q, &sessionEntity{}, map[string]any{fields.RevokedAt: now}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
