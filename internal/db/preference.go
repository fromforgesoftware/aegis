package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// The preference stores are deliberately not go-kit resource repositories.
//
// A resource.Resource carries an id, timestamps and soft-delete state, and the generic
// Getter/Lister machinery fetches one by id. A preference has none of that: its identity
// is (owner, key), it is upserted rather than created, and it is always read as a set for
// one owner. Modelling it as a resource would mean inventing a surrogate id nothing refers
// to. So these are narrow stores, in the shape of accountActiveOrgRepo.

var (
	accountPreferenceFieldMapping = map[string]string{
		fields.AccountID: "account_id",
		fields.Key:       "key",
	}
	organizationPreferenceFieldMapping = map[string]string{
		fields.OrganizationID: "organization_id",
		fields.Key:            "key",
	}
	preferenceSpecFieldMapping = map[string]string{
		fields.RealmID: "realm_id",
		fields.Key:     "key",
	}
)

type accountPreferenceEntity struct {
	EAccountID string    `gorm:"column:account_id;type:uuid;primaryKey"`
	EKey       string    `gorm:"column:key;primaryKey"`
	ERealmID   string    `gorm:"column:realm_id;type:uuid"`
	EValue     string    `gorm:"column:value"`
	EUpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (accountPreferenceEntity) TableName() string { return "aegis.account_preference" }

type organizationPreferenceEntity struct {
	EOrgID     string    `gorm:"column:organization_id;type:uuid;primaryKey"`
	EKey       string    `gorm:"column:key;primaryKey"`
	ERealmID   string    `gorm:"column:realm_id;type:uuid"`
	EValue     string    `gorm:"column:value"`
	EUpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (organizationPreferenceEntity) TableName() string { return "aegis.organization_preference" }

// preferenceSpecEntity is the declared key space, one row per (realm, key).
//
// Allowed is JSONB on the way in and out rather than a Go slice mapped by gorm, because
// gorm has no portable list type and the document it comes from is JSON already.
type preferenceSpecEntity struct {
	ERealmID   string    `gorm:"column:realm_id;type:uuid;primaryKey"`
	EKey       string    `gorm:"column:key;primaryKey"`
	EValueType string    `gorm:"column:value_type"`
	EDefault   string    `gorm:"column:default_value"`
	EAllowed   string    `gorm:"column:allowed;type:jsonb"`
	EMaxLen    int       `gorm:"column:max_len"`
	EOrgScoped bool      `gorm:"column:org_scoped"`
	EClaim     string    `gorm:"column:claim"`
	EManagedBy *string   `gorm:"column:managed_by"`
	EUpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (preferenceSpecEntity) TableName() string { return "aegis.preference_spec" }

type preferenceStore struct {
	*postgres.Repo
	orgs  *postgres.Repo
	specs *postgres.Repo
}

// NewPreferenceStore builds the store. Three field mappings means three Repo values; they
// share the same *gormdb.DBClient, so they share the connection pool and any ambient
// transaction.
func NewPreferenceStore(db *gormdb.DBClient) (*preferenceStore, error) {
	accounts, err := postgres.NewRepo(db, accountPreferenceFieldMapping)
	if err != nil {
		return nil, err
	}
	orgs, err := postgres.NewRepo(db, organizationPreferenceFieldMapping)
	if err != nil {
		return nil, err
	}
	specs, err := postgres.NewRepo(db, preferenceSpecFieldMapping)
	if err != nil {
		return nil, err
	}
	return &preferenceStore{Repo: accounts, orgs: orgs, specs: specs}, nil
}

// --- the declared key space ---------------------------------------------------------

// Specs returns the realm's declared key space, indexed by key.
func (s *preferenceStore) Specs(ctx context.Context, realmID string) (map[string]domain.PreferenceSpec, error) {
	var rows []preferenceSpecEntity
	if err := s.specs.QueryApply(ctx, query.New(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID))).Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}

	out := make(map[string]domain.PreferenceSpec, len(rows))
	for _, r := range rows {
		var allowed []string
		if r.EAllowed != "" {
			// A row whose allowed list will not parse is a corrupted spec. Treating it as
			// an empty list would silently turn an enum into a control that rejects every
			// value, so the decode failure is surfaced instead.
			if err := json.Unmarshal([]byte(r.EAllowed), &allowed); err != nil {
				return nil, postgres.NewErrUnknown(err)
			}
		}
		out[r.EKey] = domain.PreferenceSpec{
			Key:       r.EKey,
			Type:      domain.PreferenceType(r.EValueType),
			Default:   r.EDefault,
			Allowed:   allowed,
			MaxLen:    r.EMaxLen,
			OrgScoped: r.EOrgScoped,
			Claim:     r.EClaim,
		}
	}
	return out, nil
}

// UpsertSpecs writes the document's specs, stamping managed_by so reconciliation knows
// which rows it owns and never prunes one somebody created by hand.
func (s *preferenceStore) UpsertSpecs(
	ctx context.Context, realmID, managedBy string, specs []domain.PreferenceSpec,
) error {
	if len(specs) == 0 {
		return nil
	}
	rows := make([]preferenceSpecEntity, 0, len(specs))
	for _, spec := range specs {
		allowed := "[]"
		if len(spec.Allowed) > 0 {
			raw, err := json.Marshal(spec.Allowed)
			if err != nil {
				return err
			}
			allowed = string(raw)
		}
		owner := managedBy
		rows = append(rows, preferenceSpecEntity{
			ERealmID:   realmID,
			EKey:       spec.Key,
			EValueType: string(spec.Type),
			EDefault:   spec.Default,
			EAllowed:   allowed,
			EMaxLen:    spec.MaxLen,
			EOrgScoped: spec.OrgScoped,
			EClaim:     spec.Claim,
			EManagedBy: &owner,
		})
	}
	if err := s.specs.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "realm_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"value_type", "default_value", "allowed", "max_len",
			"org_scoped", "claim", "managed_by", "updated_at",
		}),
	}).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// ManagedSpecKeys lists the keys this document owns, which is the candidate set for
// pruning. A spec with a different managed_by, or none, belongs to somebody else.
func (s *preferenceStore) ManagedSpecKeys(
	ctx context.Context, realmID, managedBy string,
) ([]string, error) {
	var keys []string
	if err := s.specs.DB.WithContext(ctx).Raw(
		`SELECT key FROM aegis.preference_spec
		  WHERE realm_id = ? AND managed_by = ? ORDER BY key`,
		realmID, managedBy).Scan(&keys).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return keys, nil
}

// CountValuesForKey reports how many stored values reference a key, across both scopes.
//
// This is the prune-safety gate: deleting a spec cascades its values away, so a key people
// have actually set must not be pruned just because it left the document.
func (s *preferenceStore) CountValuesForKey(ctx context.Context, realmID, key string) (int64, error) {
	var n int64
	if err := s.specs.DB.WithContext(ctx).Raw(
		`SELECT (SELECT COUNT(*) FROM aegis.account_preference      WHERE realm_id = ? AND key = ?)
		      + (SELECT COUNT(*) FROM aegis.organization_preference WHERE realm_id = ? AND key = ?)`,
		realmID, key, realmID, key).Scan(&n).Error; err != nil {
		return 0, postgres.NewErrUnknown(err)
	}
	return n, nil
}

// DeleteSpecs removes specs, and with them — by ON DELETE CASCADE on the value tables —
// every value stored against them.
//
// The cascade is the point rather than an incidental convenience: it is what makes "no
// values for a key that no longer exists" an invariant the database holds, instead of a
// tidy-up step some future code path has to remember.
func (s *preferenceStore) DeleteSpecs(ctx context.Context, realmID string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.specs.DB.WithContext(ctx).
		Where("realm_id = ? AND key IN ?", realmID, keys).
		Delete(&preferenceSpecEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// --- stored values ------------------------------------------------------------------

// AccountPreferences returns the account's stored overrides.
//
// keys narrows the read; an empty slice reads them all. Narrowing matters because the
// common call is a settings page asking for the handful of keys it renders, and reading an
// owner's whole set to serve six of them makes the response grow with every preference ever
// declared.
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
		// No active organization is not an error: an account outside any workspace resolves
		// the spec default straight to its own override.
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
// One statement, not one per key, because a settings page saves several controls together
// and a partial failure would leave the account half-updated with no record of which half.
// The ON CONFLICT makes it idempotent, so a retried request converges rather than erroring
// on the rows that already landed.
//
// realmID is carried on the row so it can reference its spec. A value for an undeclared key
// fails the foreign key here rather than being stored and ignored later.
func (s *preferenceStore) SetAccountPreferences(
	ctx context.Context, realmID, accountID string, values map[string]string,
) error {
	if len(values) == 0 {
		return nil
	}
	rows := make([]accountPreferenceEntity, 0, len(values))
	for key, value := range values {
		rows = append(rows, accountPreferenceEntity{
			EAccountID: accountID, EKey: key, ERealmID: realmID, EValue: value,
		})
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
	ctx context.Context, realmID, orgID string, values map[string]string,
) error {
	if len(values) == 0 {
		return nil
	}
	rows := make([]organizationPreferenceEntity, 0, len(values))
	for key, value := range values {
		rows = append(rows, organizationPreferenceEntity{
			EOrgID: orgID, EKey: key, ERealmID: realmID, EValue: value,
		})
	}
	if err := s.orgs.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// DeleteAccountPreferences removes overrides, which is how "reset to default" works: the
// row goes away and the next read resolves to the organization's value or the spec's.
// Writing the default back as an explicit value would look identical to the user and be
// wrong — it would pin the value against a later change to the workspace default.
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
