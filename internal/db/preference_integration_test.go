//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

// seedPreferenceOwners creates the realm, account and organization the preference
// tables reference, and returns the two owner ids. Both tables cascade from these,
// which is what the deletion test at the bottom depends on.
func seedPreferenceOwners(t *testing.T, ctx context.Context) (accountID, orgID string) {
	t.Helper()
	client := internaltest.GetDB(t)

	realmID := uuid.NewString()
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

	return accountID, orgID
}

func TestAccountPreferencesRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })
	ctx := context.Background()

	accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{
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

	accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{"ui.theme": "dark"}))
	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{"ui.theme": "light"}))
	// Same call twice, no error and no duplicate row.
	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{"ui.theme": "light"}))

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

	accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{
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

	_, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetOrganizationPreferences(ctx, orgID, map[string]string{
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

	accountID, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{"ui.theme": "dark"}))
	require.NoError(t, store.SetOrganizationPreferences(ctx, orgID, map[string]string{"ui.theme": "light"}))

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

	accountID, _ := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetAccountPreferences(ctx, accountID, map[string]string{"ui.theme": "dark"}))
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

	accountID, orgID := seedPreferenceOwners(t, ctx)
	store, err := db.NewPreferenceStore(client)
	require.NoError(t, err)

	require.NoError(t, store.SetAccountPreferences(ctx, accountID, nil))
	require.NoError(t, store.SetOrganizationPreferences(ctx, orgID, nil))
	require.NoError(t, store.DeleteAccountPreferences(ctx, accountID, nil))
}
