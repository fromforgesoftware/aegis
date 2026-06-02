package db

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// passwordPolicyEntity backs aegis.password_policy (1 row per realm). It
// implements domain.PasswordPolicy, so reads return it directly. The realm
// id is the resource identity.
type passwordPolicyEntity struct {
	ERealmID          string    `gorm:"column:realm_id;type:uuid;primaryKey"`
	ECreatedAt        time.Time `gorm:"column:created_at;type:timestamp;autoCreateTime:true"`
	EUpdatedAt        time.Time `gorm:"column:updated_at;type:timestamp;autoUpdateTime:true"`
	EMinLength        int       `gorm:"column:min_length"`
	EMaxLength        int       `gorm:"column:max_length"`
	ERequireUppercase bool      `gorm:"column:require_uppercase"`
	ERequireLowercase bool      `gorm:"column:require_lowercase"`
	ERequireDigit     bool      `gorm:"column:require_digit"`
	ERequireSymbol    bool      `gorm:"column:require_symbol"`
}

func (e *passwordPolicyEntity) TableName() string      { return "aegis.password_policy" }
func (e *passwordPolicyEntity) ID() string             { return e.ERealmID }
func (e *passwordPolicyEntity) LID() string            { return "" }
func (e *passwordPolicyEntity) Type() resource.Type    { return domain.ResourceTypePasswordPolicy }
func (e *passwordPolicyEntity) CreatedAt() time.Time   { return e.ECreatedAt }
func (e *passwordPolicyEntity) UpdatedAt() time.Time   { return e.EUpdatedAt }
func (e *passwordPolicyEntity) DeletedAt() *time.Time  { return nil }
func (e *passwordPolicyEntity) MinLength() int         { return e.EMinLength }
func (e *passwordPolicyEntity) MaxLength() int         { return e.EMaxLength }
func (e *passwordPolicyEntity) RequireUppercase() bool { return e.ERequireUppercase }
func (e *passwordPolicyEntity) RequireLowercase() bool { return e.ERequireLowercase }
func (e *passwordPolicyEntity) RequireDigit() bool     { return e.ERequireDigit }
func (e *passwordPolicyEntity) RequireSymbol() bool    { return e.ERequireSymbol }

var passwordPolicyFieldMapping = map[string]string{
	fields.RealmID: "realm_id",
}

type passwordPolicyRepo struct {
	*postgres.Repo
}

func NewPasswordPolicyRepository(db *gormdb.DBClient) (*passwordPolicyRepo, error) {
	r, err := postgres.NewRepo(db, passwordPolicyFieldMapping)
	if err != nil {
		return nil, err
	}
	return &passwordPolicyRepo{Repo: r}, nil
}

// Get resolves a single policy from the search query (caller filters by
// realm). NotFound surfaces when the realm has no configured policy; the
// usecase falls back to the default in that case.
func (r *passwordPolicyRepo) Get(ctx context.Context, opts ...search.Option) (domain.PasswordPolicy, error) {
	s := search.New(opts...)
	var e passwordPolicyEntity
	if err := r.QueryApply(ctx, s.Query()).First(&e).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}
