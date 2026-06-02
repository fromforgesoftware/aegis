package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/slicesx"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var signingKeyFieldMapping = map[string]string{
	fields.ID:      "id",
	fields.RealmID: "realm_id",
	fields.Status:  "status",
}

// signingKeyEntity backs aegis.signing_key and implements domain.SigningKey,
// so reads return it directly. private_key holds envelope-sealed bytes.
type signingKeyEntity struct {
	postgres.Model

	ERealmID          string          `gorm:"column:realm_id;type:uuid"`
	EKid              string          `gorm:"column:kid"`
	EAlgorithm        string          `gorm:"column:algorithm"`
	EPublicJWK        json.RawMessage `gorm:"column:public_jwk;type:jsonb"`
	ESealedPrivateKey []byte          `gorm:"column:private_key"`
	EStatus           string          `gorm:"column:status"`
	ENotBefore        time.Time       `gorm:"column:not_before"`
	ENotAfter         *time.Time      `gorm:"column:not_after"`
}

func (e *signingKeyEntity) TableName() string          { return "aegis.signing_key" }
func (e *signingKeyEntity) Type() resource.Type        { return domain.ResourceTypeSigningKey }
func (e *signingKeyEntity) RealmID() string            { return e.ERealmID }
func (e *signingKeyEntity) Kid() string                { return e.EKid }
func (e *signingKeyEntity) Algorithm() string          { return e.EAlgorithm }
func (e *signingKeyEntity) PublicJWK() json.RawMessage { return e.EPublicJWK }
func (e *signingKeyEntity) Status() domain.SigningKeyStatus {
	return domain.SigningKeyStatus(e.EStatus)
}
func (e *signingKeyEntity) SealedPrivateKey() []byte { return e.ESealedPrivateKey }
func (e *signingKeyEntity) NotBefore() time.Time     { return e.ENotBefore }
func (e *signingKeyEntity) NotAfter() *time.Time     { return e.ENotAfter }

func signingKeyToEntity(k domain.SigningKey) *signingKeyEntity {
	e := &signingKeyEntity{
		Model:             postgres.ModelFromResource(k),
		ERealmID:          k.RealmID(),
		EKid:              k.Kid(),
		EAlgorithm:        k.Algorithm(),
		EPublicJWK:        k.PublicJWK(),
		ESealedPrivateKey: k.SealedPrivateKey(),
		EStatus:           string(k.Status()),
		ENotBefore:        k.NotBefore(),
		ENotAfter:         k.NotAfter(),
	}
	return e
}

type signingKeyRepo struct {
	*postgres.Repo
}

func NewSigningKeyRepository(db *gormdb.DBClient) (*signingKeyRepo, error) {
	r, err := postgres.NewRepo(db, signingKeyFieldMapping)
	if err != nil {
		return nil, err
	}
	return &signingKeyRepo{Repo: r}, nil
}

func (r *signingKeyRepo) Create(ctx context.Context, k domain.SigningKey) (domain.SigningKey, error) {
	e := signingKeyToEntity(k)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *signingKeyRepo) Get(ctx context.Context, opts ...search.Option) (domain.SigningKey, error) {
	s := search.New(opts...)
	var e signingKeyEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("signing key", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *signingKeyRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.SigningKey], error) {
	s := search.New(opts...)
	var found []*signingKeyEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(signingKeyEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *signingKeyEntity) domain.SigningKey { return e })
	return resource.NewListResponse(out, int(total)), nil
}
