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
	"github.com/fromforgesoftware/go-kit/slicesx"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var invitationFieldMapping = map[string]string{
	fields.ID:        "id",
	fields.RealmID:   "realm_id",
	fields.Email:     "email",
	fields.TokenHash: "token_hash",
	fields.Status:    "status",
}

type invitationEntity struct {
	postgres.Model

	ERealmID    string     `gorm:"column:realm_id;type:uuid"`
	EEmail      string     `gorm:"column:email"`
	EInvitedBy  *string    `gorm:"column:invited_by;type:uuid"`
	ERoleID     *string    `gorm:"column:role_id;type:uuid"`
	EResourceID *string    `gorm:"column:resource_id;type:uuid"`
	ETokenHash  string     `gorm:"column:token_hash"`
	EStatus     string     `gorm:"column:status"`
	EExpiresAt  time.Time  `gorm:"column:expires_at"`
	EAcceptedAt *time.Time `gorm:"column:accepted_at"`
}

func (e *invitationEntity) TableName() string   { return "aegis.invitation" }
func (e *invitationEntity) Type() resource.Type { return domain.ResourceTypeInvitation }
func (e *invitationEntity) RealmID() string     { return e.ERealmID }
func (e *invitationEntity) Email() string       { return e.EEmail }
func (e *invitationEntity) TokenHash() string   { return e.ETokenHash }
func (e *invitationEntity) Status() domain.InvitationStatus {
	return domain.InvitationStatus(e.EStatus)
}
func (e *invitationEntity) ExpiresAt() time.Time { return e.EExpiresAt }
func (e *invitationEntity) InvitedBy() string    { return deref(e.EInvitedBy) }
func (e *invitationEntity) RoleID() string       { return deref(e.ERoleID) }
func (e *invitationEntity) ResourceID() string   { return deref(e.EResourceID) }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func invitationToEntity(i domain.Invitation) *invitationEntity {
	return &invitationEntity{
		Model:       postgres.ModelFromResource(i),
		ERealmID:    i.RealmID(),
		EEmail:      i.Email(),
		EInvitedBy:  nilIfEmpty(i.InvitedBy()),
		ERoleID:     nilIfEmpty(i.RoleID()),
		EResourceID: nilIfEmpty(i.ResourceID()),
		ETokenHash:  i.TokenHash(),
		EStatus:     string(i.Status()),
		EExpiresAt:  i.ExpiresAt(),
	}
}

type invitationRepo struct {
	*postgres.Repo
}

func NewInvitationRepository(db *gormdb.DBClient) (*invitationRepo, error) {
	r, err := postgres.NewRepo(db, invitationFieldMapping)
	if err != nil {
		return nil, err
	}
	return &invitationRepo{Repo: r}, nil
}

func (r *invitationRepo) Create(ctx context.Context, i domain.Invitation) (domain.Invitation, error) {
	e := invitationToEntity(i)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("invitation", i.Email())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *invitationRepo) Get(ctx context.Context, opts ...search.Option) (domain.Invitation, error) {
	s := search.New(opts...)
	var e invitationEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("invitation", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *invitationRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Invitation], error) {
	s := search.New(opts...)
	var found []*invitationEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(invitationEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *invitationEntity) domain.Invitation { return e })
	return resource.NewListResponse(out, int(total)), nil
}

// GetByTokenHash resolves a pending invitation by its token hash.
func (r *invitationRepo) GetByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, error) {
	return r.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.TokenHash, tokenHash)))
}

// MarkAccepted flips status to ACCEPTED and stamps accepted_at.
func (r *invitationRepo) MarkAccepted(ctx context.Context, id string, at time.Time) error {
	if err := r.DB.WithContext(ctx).Exec(
		"UPDATE aegis.invitation SET status = 'ACCEPTED', accepted_at = ?, updated_at = NOW() WHERE id = ?",
		at, id).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
