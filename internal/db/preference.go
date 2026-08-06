package db

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/fields"
)

// The preference stores are deliberately not go-kit resource repositories.
//
// A `resource.Resource` carries an id, timestamps and soft-delete state, and the
// generic Getter/Lister machinery is built around fetching one by id. A
// preference has none of that: its identity is (owner, key), it is upserted
// rather than created, and it is always read as a set for one owner. Modelling it
// as a resource would mean inventing a surrogate id that nothing ever refers to.
// So these are narrow stores, in the shape of accountActiveOrgRepo above.

var accountPreferenceFieldMapping = map[string]string{
	fields.AccountID: "account_id",
	fields.Key:       "key",
}

var organizationPreferenceFieldMapping = map[string]string{
	fields.OrganizationID: "organization_id",
	fields.Key:            "key",
}

type accountPreferenceEntity struct {
	EAccountID string    `gorm:"column:account_id;type:uuid;primaryKey"`
	EKey       string    `gorm:"column:key;primaryKey"`
	EValue     string    `gorm:"column:value"`
	EUpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (accountPreferenceEntity) TableName() string { return "aegis.account_preference" }

type organizationPreferenceEntity struct {
	EOrgID     string    `gorm:"column:organization_id;type:uuid;primaryKey"`
	EKey       string    `gorm:"column:key;primaryKey"`
	EValue     string    `gorm:"column:value"`
	EUpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (organizationPreferenceEntity) TableName() string { return "aegis.organization_preference" }

// PreferenceStore reads and writes both preference scopes.
type preferenceStore struct {
	*postgres.Repo
	orgs *postgres.Repo
}

// NewPreferenceStore builds the store. Two field mappings means two Repo values;
// they share the same *gormdb.DBClient, so they share the connection pool and any
// ambient transaction.
func NewPreferenceStore(db *gormdb.DBClient) (*preferenceStore, error) {
	accounts, err := postgres.NewRepo(db, accountPreferenceFieldMapping)
	if err != nil {
		return nil, err
	}
	orgs, err := postgres.NewRepo(db, organizationPreferenceFieldMapping)
	if err != nil {
		return nil, err
	}
	return &preferenceStore{Repo: accounts, orgs: orgs}, nil
}

// AccountPreferences returns the account's stored overrides.
//
// keys narrows the read; an empty slice reads them all. Narrowing matters because
// the common call is a settings page asking for the handful of keys it renders,
// and reading an owner's whole set to serve six of them makes the response grow
// with every preference ever added.
func (s *preferenceStore) AccountPreferences(
	ctx context.Context, accountID string, keys []string,
) (map[string]string, error) {
	opts := []query.Option{query.FilterBy(filter.OpEq, fields.AccountID, accountID)}
	if len(keys) > 0 {
		opts = append(opts, query.FilterBy(filter.OpIn, fields.Key, keys))
	}

	var rows []accountPreferenceEntity
	if err := s.QueryApply(ctx, query.New(opts...)).Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.EKey] = r.EValue
	}
	return out, nil
}

// OrganizationPreferences returns the organization's defaults.
func (s *preferenceStore) OrganizationPreferences(
	ctx context.Context, orgID string, keys []string,
) (map[string]string, error) {
	if orgID == "" {
		// No active organization is not an error: an account outside any workspace
		// resolves registry default straight to its own override.
		return map[string]string{}, nil
	}
	opts := []query.Option{query.FilterBy(filter.OpEq, fields.OrganizationID, orgID)}
	if len(keys) > 0 {
		opts = append(opts, query.FilterBy(filter.OpIn, fields.Key, keys))
	}

	var rows []organizationPreferenceEntity
	if err := s.orgs.QueryApply(ctx, query.New(opts...)).Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.EKey] = r.EValue
	}
	return out, nil
}

// SetAccountPreferences upserts a batch in one statement.
//
// One statement, not one per key, because a settings page saves several controls
// together and a partial failure would leave the account half-updated with no
// record of which half. The ON CONFLICT makes it idempotent, so a retried request
// converges rather than erroring on the rows that already landed.
func (s *preferenceStore) SetAccountPreferences(
	ctx context.Context, accountID string, values map[string]string,
) error {
	if len(values) == 0 {
		return nil
	}
	rows := make([]accountPreferenceEntity, 0, len(values))
	for key, value := range values {
		rows = append(rows, accountPreferenceEntity{EAccountID: accountID, EKey: key, EValue: value})
	}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// SetOrganizationPreferences upserts organization defaults.
func (s *preferenceStore) SetOrganizationPreferences(
	ctx context.Context, orgID string, values map[string]string,
) error {
	if len(values) == 0 {
		return nil
	}
	rows := make([]organizationPreferenceEntity, 0, len(values))
	for key, value := range values {
		rows = append(rows, organizationPreferenceEntity{EOrgID: orgID, EKey: key, EValue: value})
	}
	if err := s.orgs.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// DeleteAccountPreferences removes overrides, which is how "reset to default"
// works: the row goes away and the next read resolves to the organization's value
// or the registry's. Writing the default back as an explicit value would look
// identical to the user and be wrong — it would pin the value against a later
// change to the workspace default.
func (s *preferenceStore) DeleteAccountPreferences(
	ctx context.Context, accountID string, keys []string,
) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.DB.WithContext(ctx).
		Where("account_id = ? AND key IN ?", accountID, keys).
		Delete(&accountPreferenceEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
