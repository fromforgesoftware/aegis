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

var externalIDPConfigFieldMapping = map[string]string{
	fields.ID:      "id",
	fields.RealmID: "realm_id",
	fields.Kind:    "kind",
	fields.Name:    "name",
	fields.Enabled: "enabled",
}

type externalIDPConfigEntity struct {
	postgres.Model

	ERealmID               string            `gorm:"column:realm_id;type:uuid"`
	EKind                  string            `gorm:"column:kind"`
	EName                  string            `gorm:"column:name"`
	EEnabled               bool              `gorm:"column:enabled"`
	EClientID              string            `gorm:"column:client_id"`
	EClientSecretEncrypted []byte            `gorm:"column:client_secret_encrypted"`
	EDiscoveryURL          string            `gorm:"column:discovery_url"`
	EIssuer                string            `gorm:"column:issuer"`
	EScopes                []string          `gorm:"column:scopes;type:jsonb;serializer:json"`
	EConfig                map[string]string `gorm:"column:config;type:jsonb;serializer:json"`
}

func (e *externalIDPConfigEntity) TableName() string   { return "aegis.external_idp_config" }
func (e *externalIDPConfigEntity) Type() resource.Type { return domain.ResourceTypeExternalIDP }
func (e *externalIDPConfigEntity) RealmID() string     { return e.ERealmID }
func (e *externalIDPConfigEntity) Kind() domain.ExternalIDPKind {
	return domain.ExternalIDPKind(e.EKind)
}
func (e *externalIDPConfigEntity) Name() string                  { return e.EName }
func (e *externalIDPConfigEntity) Enabled() bool                 { return e.EEnabled }
func (e *externalIDPConfigEntity) ClientID() string              { return e.EClientID }
func (e *externalIDPConfigEntity) ClientSecretEncrypted() []byte { return e.EClientSecretEncrypted }
func (e *externalIDPConfigEntity) DiscoveryURL() string          { return e.EDiscoveryURL }
func (e *externalIDPConfigEntity) Issuer() string                { return e.EIssuer }
func (e *externalIDPConfigEntity) Scopes() []string              { return e.EScopes }
func (e *externalIDPConfigEntity) Config() map[string]string     { return e.EConfig }

func externalIDPConfigToEntity(c domain.ExternalIDPConfig) *externalIDPConfigEntity {
	return &externalIDPConfigEntity{
		Model:                  postgres.ModelFromResource(c),
		ERealmID:               c.RealmID(),
		EKind:                  string(c.Kind()),
		EName:                  c.Name(),
		EEnabled:               c.Enabled(),
		EClientID:              c.ClientID(),
		EClientSecretEncrypted: c.ClientSecretEncrypted(),
		EDiscoveryURL:          c.DiscoveryURL(),
		EIssuer:                c.Issuer(),
		EScopes:                orEmptySlice(c.Scopes()),
		EConfig:                orEmptyMap(c.Config()),
	}
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

type externalIDPConfigRepo struct {
	*postgres.Repo
}

func NewExternalIDPConfigRepository(db *gormdb.DBClient) (*externalIDPConfigRepo, error) {
	r, err := postgres.NewRepo(db, externalIDPConfigFieldMapping)
	if err != nil {
		return nil, err
	}
	return &externalIDPConfigRepo{Repo: r}, nil
}

func (r *externalIDPConfigRepo) Create(ctx context.Context, c domain.ExternalIDPConfig) (domain.ExternalIDPConfig, error) {
	e := externalIDPConfigToEntity(c)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("external_idp_config", c.Name())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

func (r *externalIDPConfigRepo) Get(ctx context.Context, opts ...search.Option) (domain.ExternalIDPConfig, error) {
	s := search.New(opts...)
	var e externalIDPConfigEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("external_idp_config", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

func (r *externalIDPConfigRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.ExternalIDPConfig], error) {
	s := search.New(opts...)
	var found []*externalIDPConfigEntity
	if err := r.QueryApply(ctx, s.Query()).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(externalIDPConfigEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *externalIDPConfigEntity) domain.ExternalIDPConfig { return e })
	return resource.NewListResponse(out, int(total)), nil
}

func (r *externalIDPConfigRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.ExternalIDPConfig, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return nil, err
	}
	if err := r.PatchApply(ctx, q, &externalIDPConfigEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var found []*externalIDPConfigEntity
	if err := r.QueryApply(ctx, q).Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *externalIDPConfigEntity) domain.ExternalIDPConfig { return e }), nil
}

func (r *externalIDPConfigRepo) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	s := search.New(opts...)
	q := s.Query()
	if err := query.Validate(q, query.MandatoryFilters(fields.ID)); err != nil {
		return err
	}
	tx := r.QueryApply(ctx, q).Model(&externalIDPConfigEntity{})
	if delType == repository.DeleteTypeHard {
		tx = tx.Unscoped()
	}
	if err := tx.Delete(&externalIDPConfigEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
