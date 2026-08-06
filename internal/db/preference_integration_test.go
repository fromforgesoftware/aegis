//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// seedPreferenceOwners creates the realm, account and organization the preference
// tables reference, and returns the two owner ids. Both tables cascade from these,
// which is what the deletion test at the bottom depends on.
// declareKeys writes the specs the value tables reference. Values cannot exist without
// them: the foreign key makes a value for an undeclared key unrepresentable, which is the
// invariant these tests are here to hold.
func declareKeys(t *testing.T, ctx context.Context, realmID string, keys ...string) {
	t.Helper()
	client := internaltest.GetDB(t)
	for _, k := range keys {
		require.NoError(t, client.WithContext(ctx).Exec(
			`INSERT INTO aegis.preference_spec (realm_id, key, value_type, default_value, managed_by)
			 VALUES (?, ?, 'string', '', 'preferences')`, realmID, k).Error)
	}
}

func seedPreferenceOwners(t *testing.T, ctx context.Context) (realmID, accountID, orgID string) {
	t.Helper()
	client := internaltest.GetDB(t)

	realmID = uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "pref-realm").Error)

	// The email lives on user_account, not account — account holds only the
	// identity-agnostic columns. The preference tables key off account.id, so the
	// profile row is not strictly needed, but seeding a realistic account keeps this
	// suite honest against the cascade test below.
	accountID = uuid.NewString()
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.account (id, realm_id, type) VALUES (?, ?, 'USER')`,
		accountID, realmID).Error)

	// An organization needs its anchor authz resource: organization.resource_id is
	// NOT NULL, and it is how membership is expressed as bindings elsewhere.
	resourceID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.resource (id, realm_id, type) VALUES (?, ?, ?)`,
		resourceID, realmID, "organizations").Error)

	orgID = uuid.NewString()
	require.NoError(t, client.WithContext(ctx).Exec(
		`INSERT INTO aegis.organization (id, realm_id, resource_id, name, slug) VALUES (?, ?, ?, ?, ?)`,
		orgID, realmID, resourceID, "Prefs Org", "prefs-org").Error)

	return realmID, accountID, orgID
}

func TestAccountPreferencesRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{
		"ui.theme":      "dark",
		"ui.timeFormat": "12",
	}))

	all, err := store.AccountPreferences(ctx, accountID, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ui.theme": "dark", "ui.timeFormat": "12"}, all)

	// The sparse read is the common case: a settings page asks for what it renders.
	some, err := store.AccountPreferences(ctx, accountID, []string{"ui.theme"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ui.theme": "dark"}, some)

	// A key with no row is simply absent — the caller layers in the default.
	none, err := store.AccountPreferences(ctx, accountID, []string{"ui.fontSize"})
	require.NoError(t, err)
	assert.Empty(t, none)
}

// The upsert must be idempotent, because a retried request has to converge rather
// than fail on the rows that already landed.
func TestSetAccountPreferencesUpsertsIdempotently(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{"ui.theme": "dark"}))
	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{"ui.theme": "light"}))
	// Same call twice, no error and no duplicate row.
	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{"ui.theme": "light"}))

	all, err := store.AccountPreferences(ctx, accountID, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ui.theme": "light"}, all, "the last write wins, once")
}

// Deleting the override is how "reset to default" works, and it must remove the
// row rather than blank the value — a row holding "" would resolve to an empty
// preference instead of falling back.
func TestDeleteAccountPreferencesRemovesTheRow(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{
		"ui.theme":      "dark",
		"ui.timeFormat": "12",
	}))
	require.NoError(t, store.DeleteAccountPreferences(ctx, accountID, []string{"ui.theme"}))

	all, err := store.AccountPreferences(ctx, accountID, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ui.timeFormat": "12"}, all)

	// Deleting a key with no row is a no-op, not an error: a reset of something
	// already at its default is a reasonable thing for a client to ask for.
	require.NoError(t, store.DeleteAccountPreferences(ctx, accountID, []string{"ui.fontSize"}))
}

func TestOrganizationPreferencesRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, _, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetOrganizationPreferences(ctx, realmID, orgID, map[string]string{
		"ui.timeFormat": "12",
		"ui.locale":     "en-IE",
	}))

	all, err := store.OrganizationPreferences(ctx, orgID, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ui.timeFormat": "12", "ui.locale": "en-IE"}, all)

	// No active organization must be an empty result rather than an error or a
	// query with an empty uuid, which Postgres would reject as malformed.
	empty, err := store.OrganizationPreferences(ctx, "", nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// The two scopes must not read each other's rows. They are separate tables
// precisely so a workspace default and a personal override cannot be confused,
// and this is the test that would catch a copy-paste between the two methods.
func TestScopesAreIsolated(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{"ui.theme": "dark"}))
	require.NoError(t, store.SetOrganizationPreferences(ctx, realmID, orgID, map[string]string{"ui.theme": "light"}))

	fromAccount, err := store.AccountPreferences(ctx, accountID, nil)
	require.NoError(t, err)
	assert.Equal(t, "dark", fromAccount["ui.theme"])

	fromOrg, err := store.OrganizationPreferences(ctx, orgID, nil)
	require.NoError(t, err)
	assert.Equal(t, "light", fromOrg["ui.theme"])
}

// Preferences belong to their owner, so deleting the account must take them with
// it — otherwise a deleted account leaves rows that a recreated id would inherit.
func TestPreferencesCascadeWithTheirOwner(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, map[string]string{"ui.theme": "dark"}))
	require.NoError(t, client.WithContext(ctx).
		Exec(`DELETE FROM aegis.account WHERE id = ?`, accountID).Error)

	var count int64
	require.NoError(t, client.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM aegis.account_preference WHERE account_id = ?`, accountID).
		Scan(&count).Error)
	assert.Zero(t, count, "the account's preferences must not outlive it")
}

// An empty batch must not produce an INSERT with no rows, which some drivers turn
// into a syntax error.
func TestEmptyBatchesAreNoOps(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)
	declareKeys(t, ctx, realmID, "ui.theme", "ui.timeFormat", "ui.locale", "ui.fontSize")

	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID, nil))
	require.NoError(t, store.SetOrganizationPreferences(ctx, realmID, orgID, nil))
	require.NoError(t, store.DeleteAccountPreferences(ctx, accountID, nil))
}

// --- the declared key space ---------------------------------------------------------

func TestSpecsRoundTripIncludingTheEnumList(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, _, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	want := []domain.PreferenceSpec{
		{
			Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "auto",
			Allowed: []string{"light", "dark", "auto"},
		},
		{
			Key: "ui.locale", Type: domain.PreferenceTypeString, Default: "en-GB",
			MaxLen: 35, OrgScoped: true, Claim: domain.ClaimLocale,
		},
	}
	require.NoError(t, store.UpsertSpecs(ctx, realmID, "preferences", want))

	got, err := store.Specs(ctx, realmID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// The allowed list is JSONB on the way through, and a client renders its options from
	// it — an empty list would leave a select with nothing in it.
	assert.Equal(t, []string{"light", "dark", "auto"}, got["ui.theme"].Allowed)
	assert.Equal(t, domain.PreferenceTypeEnum, got["ui.theme"].Type)
	assert.Equal(t, "en-GB", got["ui.locale"].Default)
	assert.Equal(t, 35, got["ui.locale"].MaxLen)
	assert.True(t, got["ui.locale"].OrgScoped)
	assert.Equal(t, domain.ClaimLocale, got["ui.locale"].Claim)
	// A string spec must not come back with a phantom allowed list.
	assert.Empty(t, got["ui.locale"].Allowed)
}

// Reconciliation runs on every pod start, so upserting the same document repeatedly has to
// converge rather than accumulate or error.
func TestUpsertSpecsIsIdempotentAndUpdatesInPlace(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, _, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	first := []domain.PreferenceSpec{{
		Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "auto",
		Allowed: []string{"light", "dark", "auto"},
	}}
	require.NoError(t, store.UpsertSpecs(ctx, realmID, "preferences", first))
	require.NoError(t, store.UpsertSpecs(ctx, realmID, "preferences", first))

	// Extending an enum is the common evolution, and it must land on the existing row.
	second := []domain.PreferenceSpec{{
		Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "dark",
		Allowed: []string{"light", "dark", "auto", "highContrast"},
	}}
	require.NoError(t, store.UpsertSpecs(ctx, realmID, "preferences", second))

	got, err := store.Specs(ctx, realmID)
	require.NoError(t, err)
	require.Len(t, got, 1, "the same key must not accumulate rows")
	assert.Equal(t, "dark", got["ui.theme"].Default)
	assert.Len(t, got["ui.theme"].Allowed, 4)
}

// managed_by is what stops reconciliation deleting a spec it does not own.
func TestManagedSpecKeysIsScopedByOwner(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, _, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.UpsertSpecs(ctx, realmID, "preferences",
		[]domain.PreferenceSpec{{Key: "ui.mine", Type: domain.PreferenceTypeBool, Default: "true"}}))
	require.NoError(t, store.UpsertSpecs(ctx, realmID, "somebody-else",
		[]domain.PreferenceSpec{{Key: "ui.theirs", Type: domain.PreferenceTypeBool, Default: "true"}}))

	mine, err := store.ManagedSpecKeys(ctx, realmID, "preferences")
	require.NoError(t, err)
	assert.Equal(t, []string{"ui.mine"}, mine, "a spec owned by another document is not a prune candidate")
}

// THE guarantee. Deleting a spec takes every value stored against it, in the same statement,
// because the value tables reference it ON DELETE CASCADE. No cleanup pass has to remember,
// and a crash halfway cannot leave orphans behind — the database will not hold them.
func TestDeletingASpecCascadesItsValuesAway(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.UpsertSpecs(ctx, realmID, "preferences", []domain.PreferenceSpec{
		{Key: "ui.theme", Type: domain.PreferenceTypeBool, Default: "true"},
		{Key: "ui.keepme", Type: domain.PreferenceTypeBool, Default: "true"},
	}))
	require.NoError(t, store.SetAccountPreferences(ctx, realmID, accountID,
		map[string]string{"ui.theme": "false", "ui.keepme": "true"}))
	require.NoError(t, store.SetOrganizationPreferences(ctx, realmID, orgID,
		map[string]string{"ui.theme": "true"}))

	// Both scopes are counted, because both would be destroyed.
	n, err := store.CountValuesForKey(ctx, realmID, "ui.theme")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	require.NoError(t, store.DeleteSpecs(ctx, realmID, []string{"ui.theme"}))

	account, err := store.AccountPreferences(ctx, accountID, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ui.keepme": "true"}, account,
		"the pruned key's value is gone and the untouched one survives")

	org, err := store.OrganizationPreferences(ctx, orgID, nil)
	require.NoError(t, err)
	assert.Empty(t, org)

	n, err = store.CountValuesForKey(ctx, realmID, "ui.theme")
	require.NoError(t, err)
	assert.Zero(t, n)
}

// A value for a key the realm never declared must be impossible, not merely ignored on read.
// This is the constraint that makes "no rows for keys that do not exist" an invariant rather
// than a convention.
func TestAValueForAnUndeclaredKeyIsRefusedByTheDatabase(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	realmID, accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	err = store.SetAccountPreferences(ctx, realmID, accountID,
		map[string]string{"ui.neverDeclared": "x"})
	require.Error(t, err, "the foreign key must refuse a value with no spec behind it")
}
