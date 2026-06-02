package db

import (
	"context"
	"errors"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
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

var clientFieldMapping = map[string]string{
	fields.ID:       "id",
	fields.RealmID:  "realm_id",
	fields.ClientID: "client_id",
}

// clientEntity backs aegis.client and implements domain.Client, so reads
// return it directly. The raw secret is never stored — only client_secret_hash.
type clientEntity struct {
	postgres.Model

	ERealmID          string   `gorm:"column:realm_id;type:uuid"`
	EClientID         string   `gorm:"column:client_id"`
	EClientSecretHash string   `gorm:"column:client_secret_hash"`
	EType             string   `gorm:"column:type"`
	EName             string   `gorm:"column:name"`
	EGrantTypes       []string `gorm:"column:grant_types;type:jsonb;serializer:json"`
	EScopes           []string `gorm:"column:scopes;type:jsonb;serializer:json"`
	ERedirectURIs     []string `gorm:"column:redirect_uris;type:jsonb;serializer:json"`
	EPKCERequired     bool     `gorm:"column:pkce_required"`
}

func (e *clientEntity) TableName() string             { return "aegis.client" }
func (e *clientEntity) Type() resource.Type           { return domain.ResourceTypeClient }
func (e *clientEntity) RealmID() string               { return e.ERealmID }
func (e *clientEntity) ClientID() string              { return e.EClientID }
func (e *clientEntity) ClientType() domain.ClientType { return domain.ClientType(e.EType) }
func (e *clientEntity) Name() string                  { return e.EName }
func (e *clientEntity) GrantTypes() []string          { return e.EGrantTypes }
func (e *clientEntity) Scopes() []string              { return e.EScopes }
func (e *clientEntity) RedirectURIs() []string        { return e.ERedirectURIs }
func (e *clientEntity) PKCERequired() bool            { return e.EPKCERequired }
func (e *clientEntity) SecretHash() string            { return e.EClientSecretHash }
func (e *clientEntity) Secret() string                { return "" } // never read back

func clientToEntity(c domain.Client) *clientEntity {
	return &clientEntity{
		Model:             postgres.ModelFromResource(c),
		ERealmID:          c.RealmID(),
		EClientID:         c.ClientID(),
		EClientSecretHash: c.SecretHash(),
		EType:             string(c.ClientType()),
		EName:             c.Name(),
		// Coerce nil → [] so the JSONB columns store '[]' (NOT NULL), not null.
		EGrantTypes:   orEmptySlice(c.GrantTypes()),
		EScopes:       orEmptySlice(c.Scopes()),
		ERedirectURIs: orEmptySlice(c.RedirectURIs()),
		EPKCERequired: c.PKCERequired(),
	}
}

func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// nilIfEmpty maps "" to NULL so an optional UUID FK column isn't written as an empty string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type clientRepo struct {
	*postgres.Repo
}

func NewClientRepository(db *gormdb.DBClient) (*clientRepo, error) {
	r, err := postgres.NewRepo(db, clientFieldMapping)
	if err != nil {
		return nil, err
	}
	return &clientRepo{Repo: r}, nil
}

func (r *clientRepo) Create(ctx context.Context, c domain.Client) (domain.Client, error) {
	e := clientToEntity(c)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("client", c.ClientID())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *clientRepo) Get(ctx context.Context, opts ...search.Option) (domain.Client, error) {
	s := search.New(opts...)
	var e clientEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("client", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *clientRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.Client], error) {
	s := search.New(opts...)
	var found []*clientEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(clientEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *clientEntity) domain.Client { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *clientRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&clientEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&clientEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
