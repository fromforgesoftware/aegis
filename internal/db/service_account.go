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

var serviceAccountFieldMapping = map[string]string{
	fields.ID:       "account_id",
	fields.RealmID:  "realm_id",
	fields.ClientID: "client_id",
}

// serviceAccountEntity backs aegis.service_account. The account_id is the
// primary key and the resource id, so the row implements resource.Resource
// with account_id as its identity.
type serviceAccountEntity struct {
	EAccountID  string     `gorm:"column:account_id;type:uuid;primaryKey"`
	ECreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	EUpdatedAt  time.Time  `gorm:"column:updated_at;autoUpdateTime:true"`
	ERealmID    string     `gorm:"column:realm_id;type:uuid"`
	EName       string     `gorm:"column:name"`
	EClientID   string     `gorm:"column:client_id"`
	ESecretHash string     `gorm:"column:secret_hash"`
	EScopes     []string   `gorm:"column:scopes;type:jsonb;serializer:json"`
	ELastUsedAt *time.Time `gorm:"column:last_used_at"`
}

func (e *serviceAccountEntity) TableName() string      { return "aegis.service_account" }
func (e *serviceAccountEntity) Type() resource.Type    { return domain.ResourceTypeServiceAccount }
func (e *serviceAccountEntity) ID() string             { return e.EAccountID }
func (e *serviceAccountEntity) LID() string            { return "" }
func (e *serviceAccountEntity) CreatedAt() time.Time   { return e.ECreatedAt }
func (e *serviceAccountEntity) UpdatedAt() time.Time   { return e.EUpdatedAt }
func (e *serviceAccountEntity) DeletedAt() *time.Time  { return nil }
func (e *serviceAccountEntity) RealmID() string        { return e.ERealmID }
func (e *serviceAccountEntity) Name() string           { return e.EName }
func (e *serviceAccountEntity) ClientID() string       { return e.EClientID }
func (e *serviceAccountEntity) Scopes() []string       { return e.EScopes }
func (e *serviceAccountEntity) LastUsedAt() *time.Time { return e.ELastUsedAt }

type serviceAccountRepo struct {
	*postgres.Repo
}

func NewServiceAccountRepository(db *gormdb.DBClient) (*serviceAccountRepo, error) {
	r, err := postgres.NewRepo(db, serviceAccountFieldMapping)
	if err != nil {
		return nil, err
	}
	return &serviceAccountRepo{Repo: r}, nil
}

// Create inserts the service_account row carrying the secret hash. The account
// id must already exist (the usecase creates the SERVICE account first, in the
// same transaction).
func (r *serviceAccountRepo) Create(ctx context.Context, sa domain.ServiceAccount, secretHash string) (domain.ServiceAccount, error) {
	e := &serviceAccountEntity{
		EAccountID:  sa.ID(),
		ERealmID:    sa.RealmID(),
		EName:       sa.Name(),
		EClientID:   sa.ClientID(),
		ESecretHash: secretHash,
		EScopes:     orEmptySlice(sa.Scopes()),
	}
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

// GetByClientID resolves a service account by realm + client_id, returning its
// stored secret hash for authentication. NotFound when no row matches.
func (r *serviceAccountRepo) GetByClientID(ctx context.Context, realmID, clientID string) (domain.ServiceAccount, string, error) {
	q := query.New(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.ClientID, clientID),
	)
	var e serviceAccountEntity
	if err := r.QueryApply(ctx, q).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", apierrors.NotFound("service account", clientID)
		}
		return nil, "", postgres.NewErrUnknown(err)
	}
	return &e, e.ESecretHash, nil
}

func (r *serviceAccountRepo) Get(ctx context.Context, opts ...search.Option) (domain.ServiceAccount, error) {
	s := search.New(opts...)
	var e serviceAccountEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("service account", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *serviceAccountRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.ServiceAccount], error) {
	q := search.New(opts...).Query()
	var found []*serviceAccountEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(serviceAccountEntity), q).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *serviceAccountEntity) domain.ServiceAccount { return e })
	return resource.NewListResponse(out, int(total)), nil
}

// Delete removes the service_account row; the SERVICE account itself is
// soft-deleted by the usecase.
func (r *serviceAccountRepo) Delete(ctx context.Context, accountID string) error {
	if err := r.DB.WithContext(ctx).
		Exec("DELETE FROM aegis.service_account WHERE account_id = ?", accountID).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// TouchLastUsed stamps last_used_at after a successful token issue.
func (r *serviceAccountRepo) TouchLastUsed(ctx context.Context, accountID string, at time.Time) error {
	if err := r.DB.WithContext(ctx).
		Exec("UPDATE aegis.service_account SET last_used_at = ?, updated_at = NOW() WHERE account_id = ?", at, accountID).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
