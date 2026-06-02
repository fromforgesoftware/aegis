package db

import (
	"context"
	"errors"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/slicesx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var sessionStateFieldMapping = map[string]string{
	fields.ID:         "session_id",
	fields.AccountID:  "account_id",
	fields.LastActive: "last_active",
}

type sessionStateEntity struct {
	ESessionID      string    `gorm:"column:session_id;type:uuid;primaryKey"`
	ECreatedAt      time.Time `gorm:"column:created_at;autoCreateTime:true"`
	EUpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime:true"`
	EAccountID      string    `gorm:"column:account_id;type:uuid"`
	ECurrentRealmID *string   `gorm:"column:current_realm_id;type:uuid"`
	ECurrentShard   string    `gorm:"column:current_shard"`
	ERegion         string    `gorm:"column:region"`
	EIP             string    `gorm:"column:ip"`
	EUserAgent      string    `gorm:"column:user_agent"`
	ELastActive     time.Time `gorm:"column:last_active"`
}

func (sessionStateEntity) TableName() string        { return "aegis.session_state" }
func (e *sessionStateEntity) ID() string            { return e.ESessionID }
func (e *sessionStateEntity) LID() string           { return "" }
func (e *sessionStateEntity) Type() resource.Type   { return domain.ResourceTypeSessionState }
func (e *sessionStateEntity) CreatedAt() time.Time  { return e.ECreatedAt }
func (e *sessionStateEntity) UpdatedAt() time.Time  { return e.EUpdatedAt }
func (e *sessionStateEntity) DeletedAt() *time.Time { return nil }
func (e *sessionStateEntity) AccountID() string     { return e.EAccountID }
func (e *sessionStateEntity) CurrentRealmID() string {
	if e.ECurrentRealmID == nil {
		return ""
	}
	return *e.ECurrentRealmID
}
func (e *sessionStateEntity) CurrentShard() string  { return e.ECurrentShard }
func (e *sessionStateEntity) Region() string        { return e.ERegion }
func (e *sessionStateEntity) IP() string            { return e.EIP }
func (e *sessionStateEntity) UserAgent() string     { return e.EUserAgent }
func (e *sessionStateEntity) LastActive() time.Time { return e.ELastActive }

func sessionStateToEntity(s domain.SessionState) *sessionStateEntity {
	return &sessionStateEntity{
		ESessionID:      s.ID(),
		EAccountID:      s.AccountID(),
		ECurrentRealmID: nilIfEmpty(s.CurrentRealmID()),
		ECurrentShard:   s.CurrentShard(),
		ERegion:         s.Region(),
		EIP:             s.IP(),
		EUserAgent:      s.UserAgent(),
		ELastActive:     s.LastActive(),
	}
}

type sessionStateRepo struct {
	*postgres.Repo
}

func NewSessionStateRepository(db *gormdb.DBClient) (*sessionStateRepo, error) {
	r, err := postgres.NewRepo(db, sessionStateFieldMapping)
	if err != nil {
		return nil, err
	}
	return &sessionStateRepo{Repo: r}, nil
}

// Upsert writes the session's current topology, refreshing last_active.
func (r *sessionStateRepo) Upsert(ctx context.Context, s domain.SessionState) (domain.SessionState, error) {
	e := sessionStateToEntity(s)
	if e.ELastActive.IsZero() {
		e.ELastActive = time.Now()
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"account_id", "current_realm_id", "current_shard", "region", "ip", "user_agent", "last_active", "updated_at",
		}),
	}).Create(e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *sessionStateRepo) Get(ctx context.Context, opts ...search.Option) (domain.SessionState, error) {
	s := search.New(opts...)
	var e sessionStateEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("session state", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *sessionStateRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.SessionState], error) {
	s := search.New(opts...)
	var found []*sessionStateEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(sessionStateEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *sessionStateEntity) domain.SessionState { return e })
	return resource.NewListResponse(out, int(total)), nil
}

// Touch refreshes last_active for a single session.
func (r *sessionStateRepo) Touch(ctx context.Context, sessionID string, at time.Time) error {
	if err := r.DB.WithContext(ctx).
		Exec("UPDATE aegis.session_state SET last_active = ?, updated_at = NOW() WHERE session_id = ?", at, sessionID).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// PurgeIdle deletes session-state rows untouched since before, returning the
// count removed.
func (r *sessionStateRepo) PurgeIdle(ctx context.Context, before time.Time) (int64, error) {
	tx := r.DB.WithContext(ctx).
		Exec("DELETE FROM aegis.session_state WHERE last_active < ?", before)
	if tx.Error != nil {
		return 0, postgres.NewErrUnknown(tx.Error)
	}
	return tx.RowsAffected, nil
}
