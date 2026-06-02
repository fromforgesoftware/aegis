// Package db holds Aegis's persistence layer: GORM entities, the
// domain→entity encoders, and the repositories implementing the
// app-layer ports.
package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/slicesx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// pgUniqueViolation is Postgres SQLSTATE 23505.
const pgUniqueViolation = "23505"

// userAccountJoin attaches the profile table so a query can filter on
// email (which lives on aegis.user_account, not aegis.account). The
// fieldMapping points fields.Email at this alias.
const userAccountJoin = "JOIN aegis.user_account ua ON ua.account_id = aegis.account.id"

// accountFieldMapping maps logical field names to DB columns for the kit
// query DSL. fields.Email is qualified to the join alias since it lives
// on the profile table.
var accountFieldMapping = map[string]string{
	fields.ID:               "id",
	fields.RealmID:          "realm_id",
	fields.Status:           "status",
	fields.Type:             "type",
	fields.AccountID:        "account_id",
	fields.Email:            "ua.email",
	fields.EmailVerified:    "email_verified",
	fields.LastLoginAt:      "last_login_at",
	fields.FailedLoginCount: "failed_login_count",
	fields.LockedUntil:      "locked_until",
}

var credentialFieldMapping = map[string]string{
	fields.AccountID: "account_id",
}

// -----------------------------------------------------------------------------
// Entities
// -----------------------------------------------------------------------------

// accountEntity is the GORM root for the Account aggregate. Its profile
// (aegis.user_account) hangs off it via a foreign-key relation; the two
// load and write together. realm_id lives here (the aggregate root);
// children derive it. The entity implements domain.Account, so reads
// return it directly.
type accountEntity struct {
	postgres.Model

	ERealmID          string     `gorm:"column:realm_id;type:uuid"`
	EStatus           string     `gorm:"column:status"`
	EType             string     `gorm:"column:type"`
	ELastLoginAt      *time.Time `gorm:"column:last_login_at"`
	EFailedLoginCount int        `gorm:"column:failed_login_count"`
	ELockedUntil      *time.Time `gorm:"column:locked_until"`

	// Profile (aegis.user_account) loads with the aggregate. The password
	// credential is owned by credentialRepo, not the Account aggregate.
	Profile *userAccountEntity `gorm:"foreignKey:EAccountID;references:EID"`
}

func (e *accountEntity) TableName() string               { return "aegis.account" }
func (e *accountEntity) Type() resource.Type             { return domain.ResourceTypeAccount }
func (e *accountEntity) RealmID() string                 { return e.ERealmID }
func (e *accountEntity) Status() domain.AccountStatus    { return domain.AccountStatus(e.EStatus) }
func (e *accountEntity) AccountType() domain.AccountType { return domain.AccountType(e.EType) }

func (e *accountEntity) Email() string {
	if e.Profile == nil {
		return ""
	}
	return e.Profile.EEmail
}

func (e *accountEntity) EmailVerified() bool {
	if e.Profile == nil {
		return false
	}
	return e.Profile.EEmailVerified
}

func (e *accountEntity) DisplayName() string {
	if e.Profile == nil {
		return ""
	}
	return e.Profile.EDisplayName
}

func (e *accountEntity) PhotoURL() string {
	if e.Profile == nil {
		return ""
	}
	return e.Profile.EPhotoURL
}

func (e *accountEntity) LastLoginAt() *time.Time { return e.ELastLoginAt }
func (e *accountEntity) FailedLoginCount() int   { return e.EFailedLoginCount }
func (e *accountEntity) LockedUntil() *time.Time { return e.ELockedUntil }

// userAccountEntity backs aegis.user_account (1:1 with account).
type userAccountEntity struct {
	EAccountID     string     `gorm:"column:account_id;type:uuid;primaryKey"`
	ECreatedAt     time.Time  `gorm:"column:created_at;type:timestamp;autoCreateTime:true"`
	EUpdatedAt     time.Time  `gorm:"column:updated_at;type:timestamp;autoUpdateTime:true"`
	EDeletedAt     *time.Time `gorm:"column:deleted_at;type:timestamp"`
	EEmail         string     `gorm:"column:email"`
	EEmailVerified bool       `gorm:"column:email_verified"`
	EDisplayName   string     `gorm:"column:display_name"`
	EPhotoURL      string     `gorm:"column:photo_url"`
}

func (e *userAccountEntity) TableName() string { return "aegis.user_account" }

// passwordCredentialEntity backs aegis.password_credential (1:1).
type passwordCredentialEntity struct {
	EAccountID string          `gorm:"column:account_id;type:uuid;primaryKey"`
	ECreatedAt time.Time       `gorm:"column:created_at;type:timestamp;autoCreateTime:true"`
	EUpdatedAt time.Time       `gorm:"column:updated_at;type:timestamp;autoUpdateTime:true"`
	EHash      string          `gorm:"column:hash"`
	EAlgo      string          `gorm:"column:algo"`
	EParams    json.RawMessage `gorm:"column:params;type:jsonb"`
}

func (e *passwordCredentialEntity) TableName() string { return "aegis.password_credential" }

// -----------------------------------------------------------------------------
// Domain → entity mappers
// -----------------------------------------------------------------------------

// accountToEntity maps the Account aggregate to its GORM entity (account +
// profile). The id comes from the resource (empty on create → the uuid
// default fills it).
func accountToEntity(acc domain.Account) *accountEntity {
	return &accountEntity{
		Model:             postgres.ModelFromResource(acc),
		ERealmID:          acc.RealmID(),
		EStatus:           string(acc.Status()),
		EType:             string(acc.AccountType()),
		ELastLoginAt:      acc.LastLoginAt(),
		EFailedLoginCount: acc.FailedLoginCount(),
		ELockedUntil:      acc.LockedUntil(),
		Profile: &userAccountEntity{
			EEmail:         acc.Email(),
			EEmailVerified: acc.EmailVerified(),
			EDisplayName:   acc.DisplayName(),
			EPhotoURL:      acc.PhotoURL(),
		},
	}
}

// credentialToEntity maps a hashed password to its GORM entity.
func credentialToEntity(accountID string, cred app.HashedPassword) (*passwordCredentialEntity, error) {
	params, err := json.Marshal(cred.Params)
	if err != nil {
		return nil, apierrors.InternalError("failed to encode credential params")
	}
	return &passwordCredentialEntity{
		EAccountID: accountID,
		EHash:      cred.Encoded,
		EAlgo:      cred.Algo,
		EParams:    params,
	}, nil
}

// -----------------------------------------------------------------------------
// Account repository
// -----------------------------------------------------------------------------

type accountRepo struct {
	*postgres.Repo
}

// NewAccountRepository builds the Account repository over the kit's
// generic postgres.Repo. fx binds the concrete type to
// app.AccountRepository.
func NewAccountRepository(db *gormdb.DBClient) (*accountRepo, error) {
	r, err := postgres.NewRepo(db, accountFieldMapping)
	if err != nil {
		return nil, err
	}
	return &accountRepo{Repo: r}, nil
}

// Create inserts the account + its profile association. Inserts stay raw
// GORM (QueryApply is for read/update/delete filtering). The password
// credential is written separately by credentialRepo within the
// registration transaction.
func (r *accountRepo) Create(ctx context.Context, acc domain.Account) (domain.Account, error) {
	e := accountToEntity(acc)
	if err := r.DB.WithContext(ctx).Create(e).Error; err != nil {
		if postgres.ErrorIs(err, pgUniqueViolation) {
			return nil, apierrors.AlreadyExists("account", acc.Email())
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return e, nil
}

// Get implements repository.Getter: it resolves a single Account from the
// search query (caller supplies filters via search.WithQueryOpts). The
// profile is always preloaded; the user_account join is added only when
// the query filters on email (which lives on the profile table).
func (r *accountRepo) Get(ctx context.Context, opts ...search.Option) (domain.Account, error) {
	q := search.New(opts...).Query()
	// Require ≥1 filter and reject any field outside the allow-list, so a
	// stray/unknown filter is a clean 400 rather than a Postgres
	// "column does not exist" 500 (and never an unfiltered scan).
	if err := query.Validate(q,
		query.AtLeastOneFilter(),
		query.OptionalFilters(fields.ID, fields.RealmID, fields.Email),
		query.ValidFilter(fields.ID, filter.ValidateTyped[string], filter.ValidateUUID),
		query.ValidFilter(fields.RealmID, filter.ValidateTyped[string], filter.ValidateUUID),
		query.ValidFilter(fields.Email, filter.ValidateTyped[string], filter.ValidateNotZero),
	); err != nil {
		return nil, err
	}
	tx := r.QueryApply(ctx, q).Preload("Profile")
	tx = r.applyProfileJoin(tx, q)

	var e accountEntity
	if err := tx.First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierrors.NotFound("account", "")
		}
		return nil, postgres.NewErrUnknown(err)
	}
	return &e, nil
}

// applyProfileJoin adds the user_account join when the query filters on a
// profile-table column (email). The kit query DSL doesn't model joins, so
// we add it by hand — same approach as trading-bot's join filters.
func (r *accountRepo) applyProfileJoin(tx *gorm.DB, q query.Query) *gorm.DB {
	if _, ok := q.Filters()[fields.Email]; ok {
		tx = tx.Joins(userAccountJoin)
	}
	return tx
}

// Patch implements repository.Patcher: it applies the patch fields to the
// account-root table for the rows matching the search filter, then
// returns the updated aggregates. last_login_at, status, bans, etc. all
// live on aegis.account, so they patch through here; profile-table fields
// would need their own path.
func (r *accountRepo) Patch(ctx context.Context, opts ...repository.PatchOption) ([]domain.Account, error) {
	p := repository.NewPatchQuery(opts...)
	q := search.New(p.SearchOpts()...).Query()

	// A mandatory id filter prevents an unbounded UPDATE (no WHERE → every
	// row). Patch targets a single account by primary key.
	if err := query.Validate(q,
		query.MandatoryFilters(fields.ID),
		query.ValidFilter(fields.ID, filter.ValidateTyped[string], filter.ValidateUUID),
	); err != nil {
		return nil, err
	}

	if err := r.PatchApply(ctx, q, &accountEntity{}, p.PatchFields()).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}

	var found []*accountEntity
	if err := r.QueryApply(ctx, q).Preload("Profile").Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return slicesx.Map(found, func(e *accountEntity) domain.Account { return e }), nil
}

// MarkEmailVerified flips email_verified on the profile table. It's a
// dedicated method because email_verified lives on aegis.user_account, not
// the account root that the generic Patcher targets.
func (r *accountRepo) MarkEmailVerified(ctx context.Context, accountID string) error {
	q := query.New(query.FilterBy(filter.OpEq, fields.AccountID, accountID))
	if err := r.PatchApply(ctx, q, &userAccountEntity{}, map[string]any{fields.EmailVerified: true}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Ban sets the account BANNED with an optional expiry + reason. A nil until is
// a permanent ban (the sweeper never restores it).
func (r *accountRepo) Ban(ctx context.Context, accountID string, until *time.Time, reason string) error {
	if err := r.DB.WithContext(ctx).Exec(
		"UPDATE aegis.account SET status = 'BANNED', banned_until = ?, ban_reason = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
		until, reason, accountID).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Unban restores a banned account to ENABLED and clears the ban fields.
func (r *accountRepo) Unban(ctx context.Context, accountID string) error {
	if err := r.DB.WithContext(ctx).Exec(
		"UPDATE aegis.account SET status = 'ENABLED', banned_until = NULL, ban_reason = NULL, updated_at = NOW() WHERE id = ? AND status = 'BANNED'",
		accountID).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// RestoreExpiredBans flips temporary bans whose banned_until has passed back to
// ENABLED, returning how many were restored. Permanent bans (NULL until) stay.
func (r *accountRepo) RestoreExpiredBans(ctx context.Context, now time.Time) (int64, error) {
	tx := r.DB.WithContext(ctx).Exec(
		"UPDATE aegis.account SET status = 'ENABLED', banned_until = NULL, ban_reason = NULL, updated_at = NOW() WHERE status = 'BANNED' AND banned_until IS NOT NULL AND banned_until <= ?",
		now)
	if tx.Error != nil {
		return 0, postgres.NewErrUnknown(tx.Error)
	}
	return tx.RowsAffected, nil
}

// -----------------------------------------------------------------------------
// Credential repository
// -----------------------------------------------------------------------------

type credentialRepo struct {
	*postgres.Repo
}

func NewCredentialRepository(db *gormdb.DBClient) (*credentialRepo, error) {
	r, err := postgres.NewRepo(db, credentialFieldMapping)
	if err != nil {
		return nil, err
	}
	return &credentialRepo{Repo: r}, nil
}

// SetPassword upserts the password credential for an account (so password
// change/reset reuses the same path).
func (r *credentialRepo) SetPassword(ctx context.Context, accountID string, cred app.HashedPassword) error {
	e, err := credentialToEntity(accountID, cred)
	if err != nil {
		return err
	}
	tx := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"hash", "algo", "params", "updated_at"}),
	})
	if err := tx.Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

func (r *credentialRepo) GetPasswordHash(ctx context.Context, accountID string) (string, error) {
	var e passwordCredentialEntity
	q := query.New(query.FilterBy(filter.OpEq, fields.AccountID, accountID))
	err := r.QueryApply(ctx, q).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", apierrors.NotFound("credential", accountID)
		}
		return "", postgres.NewErrUnknown(err)
	}
	return e.EHash, nil
}
