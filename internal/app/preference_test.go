package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

const (
	testRealmID   = "33333333-3333-4333-8333-333333333333"
	testAccountID = "11111111-1111-4111-8111-111111111111"
	testOrgID     = "22222222-2222-4222-8222-222222222222"
)

func newPreferenceUsecase(t *testing.T) (
	*apptest.PreferenceStore, *apptest.AccountActiveOrgRepository, app.PreferenceUsecase,
) {
	store := apptest.NewPreferenceStore(t)
	active := apptest.NewAccountActiveOrgRepository(t)
	return store, active, app.NewPreferenceUsecase(store, active)
}

func themeSpec() domain.PreferenceSpec {
	return domain.PreferenceSpec{
		Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "auto",
		Allowed: []string{"light", "dark", "auto"},
	}
}

func timeFormatSpec() domain.PreferenceSpec {
	return domain.PreferenceSpec{
		Key: "ui.timeFormat", Type: domain.PreferenceTypeEnum, Default: "24",
		Allowed: []string{"12", "24"}, OrgScoped: true,
	}
}

func declared() map[string]domain.PreferenceSpec {
	return map[string]domain.PreferenceSpec{
		"ui.theme":      themeSpec(),
		"ui.timeFormat": timeFormatSpec(),
	}
}

// --- reconciliation -----------------------------------------------------------------

// Apply upserts what the document declares, then prunes what left it — in that order, so a
// key that moved namespace is never absent from both.
func TestApplyUpsertsThenPrunes(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	document := domain.PreferenceDocument{Specs: []domain.PreferenceSpec{themeSpec()}}

	store.EXPECT().UpsertSpecs(mock.Anything, testRealmID, app.PreferenceManagedBy,
		document.Specs).Return(nil).Once()
	store.EXPECT().ManagedSpecKeys(mock.Anything, testRealmID, app.PreferenceManagedBy).
		Return([]string{"ui.theme", "ui.retired"}, nil).Once()
	store.EXPECT().CountValuesForKey(mock.Anything, testRealmID, "ui.retired").
		Return(int64(0), nil).Once()
	store.EXPECT().DeleteSpecs(mock.Anything, testRealmID, []string{"ui.retired"}).
		Return(nil).Once()

	require.NoError(t, uc.Apply(context.Background(), testRealmID, document))
}

// THE prune gate. Removing a key deletes every value stored against it through the spec's
// cascade, so a key people have actually set must not disappear just because it left the
// document — a slimmed config, or a typo, would otherwise destroy settings silently.
func TestApplyRefusesToPruneAKeyWithStoredValues(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	document := domain.PreferenceDocument{Specs: []domain.PreferenceSpec{themeSpec()}}

	store.EXPECT().UpsertSpecs(mock.Anything, testRealmID, app.PreferenceManagedBy,
		mock.Anything).Return(nil).Once()
	store.EXPECT().ManagedSpecKeys(mock.Anything, testRealmID, app.PreferenceManagedBy).
		Return([]string{"ui.theme", "ui.timeFormat"}, nil).Once()
	store.EXPECT().CountValuesForKey(mock.Anything, testRealmID, "ui.timeFormat").
		Return(int64(7), nil).Once()

	err := uc.Apply(context.Background(), testRealmID, document)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ui.timeFormat")
	assert.Contains(t, err.Error(), "7 stored values")
	// The message has to say how to proceed deliberately, or the only way past it is guesswork.
	assert.Contains(t, err.Error(), "force")
	store.AssertNotCalled(t, "DeleteSpecs")
}

// force is the deliberate override, and it must skip the count entirely — the operator has
// already accepted the loss, so asking the database again only invites a different answer.
func TestApplyPrunesWhenForced(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	document := domain.PreferenceDocument{
		Specs: []domain.PreferenceSpec{themeSpec()},
		Force: []string{"ui.timeFormat"},
	}

	store.EXPECT().UpsertSpecs(mock.Anything, testRealmID, app.PreferenceManagedBy,
		mock.Anything).Return(nil).Once()
	store.EXPECT().ManagedSpecKeys(mock.Anything, testRealmID, app.PreferenceManagedBy).
		Return([]string{"ui.theme", "ui.timeFormat"}, nil).Once()
	store.EXPECT().DeleteSpecs(mock.Anything, testRealmID, []string{"ui.timeFormat"}).
		Return(nil).Once()

	require.NoError(t, uc.Apply(context.Background(), testRealmID, document))
	store.AssertNotCalled(t, "CountValuesForKey")
}

// An invalid document must be refused before anything is written, because provisioning is
// fatal and a half-applied key space is worse than none.
func TestApplyRefusesAnInvalidDocument(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	err := uc.Apply(context.Background(), testRealmID, domain.PreferenceDocument{
		Specs: []domain.PreferenceSpec{
			{Key: "nonamespace", Type: domain.PreferenceTypeBool, Default: "true"},
		},
	})
	require.Error(t, err)
	store.AssertNotCalled(t, "UpsertSpecs")
}

func TestApplyRequiresARealm(t *testing.T) {
	_, _, uc := newPreferenceUsecase(t)

	err := uc.Apply(context.Background(), "", domain.PreferenceDocument{
		Specs: []domain.PreferenceSpec{themeSpec()},
	})
	assert.Error(t, err)
}

// --- resolution ---------------------------------------------------------------------

func TestResolveLayersTheThreeSources(t *testing.T) {
	store, active, uc := newPreferenceUsecase(t)
	keys := []string{"ui.theme", "ui.timeFormat"}

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	active.EXPECT().Get(mock.Anything, testAccountID).Return(testOrgID, "admin", true, nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, keys).
		Return(map[string]string{"ui.theme": "dark"}, nil).Once()
	store.EXPECT().OrganizationPreferences(mock.Anything, testOrgID, keys).
		Return(map[string]string{"ui.timeFormat": "12"}, nil).Once()

	got, err := uc.Resolve(context.Background(), testRealmID, testAccountID, keys)
	require.NoError(t, err)

	byKey := map[string]domain.Preference{}
	for _, p := range got {
		byKey[p.Key] = p
	}
	assert.Equal(t, "dark", byKey["ui.theme"].Value)
	assert.Equal(t, domain.PreferenceSourceAccount, byKey["ui.theme"].Source)
	assert.Equal(t, "12", byKey["ui.timeFormat"].Value)
	assert.Equal(t, domain.PreferenceSourceOrganization, byKey["ui.timeFormat"].Source)
}

// Preferences are read on every page load, so a fault in the unrelated active-org lookup
// must degrade to "no workspace layer" rather than break the settings page.
func TestResolveDegradesWhenTheActiveOrgLookupFails(t *testing.T) {
	store, active, uc := newPreferenceUsecase(t)

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	active.EXPECT().Get(mock.Anything, testAccountID).
		Return("", "", false, errors.New("active org table unavailable")).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string{"ui.theme"}).
		Return(map[string]string{"ui.theme": "light"}, nil).Once()

	got, err := uc.Resolve(context.Background(), testRealmID, testAccountID, []string{"ui.theme"})
	require.NoError(t, err, "a failed org lookup must not fail the read")
	require.Len(t, got, 1)
	assert.Equal(t, "light", got[0].Value)
}

// A key this realm has not declared is refused, not silently dropped: a blank control looks
// like a backend fault, while a named error points at the typo.
func TestResolveRefusesUndeclaredKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()

	_, err := uc.Resolve(context.Background(), testRealmID, testAccountID, []string{"ui.them"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ui.them")
}

func TestResolveRequiresRealmAndAccount(t *testing.T) {
	_, _, uc := newPreferenceUsecase(t)

	_, err := uc.Resolve(context.Background(), "", testAccountID, nil)
	assert.Error(t, err)
	_, err = uc.Resolve(context.Background(), testRealmID, "", nil)
	assert.Error(t, err)
}

// --- writes -------------------------------------------------------------------------

func TestSetForAccountValidatesAgainstTheRealmsSpecs(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{name: "undeclared key", values: map[string]string{"ui.wallpaper": "beach"}, want: "ui.wallpaper"},
		{name: "value outside the enum", values: map[string]string{"ui.theme": "solarized"}, want: "ui.theme"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, _, uc := newPreferenceUsecase(t)
			store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()

			err := uc.SetForAccount(context.Background(), testRealmID, testAccountID, tc.values)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			store.AssertNotCalled(t, "SetAccountPreferences")
		})
	}
}

// The batch is all-or-nothing: one bad value must stop the good ones, or the page shows some
// changes applied and some not with no way to tell which.
func TestSetForAccountRejectsAWholeBatchForOneBadValue(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()

	err := uc.SetForAccount(context.Background(), testRealmID, testAccountID, map[string]string{
		"ui.theme":      "dark",
		"ui.timeFormat": "half-past",
	})
	require.Error(t, err)
	store.AssertNotCalled(t, "SetAccountPreferences")
}

func TestSetForAccountWritesAValidBatch(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string(nil)).
		Return(map[string]string{}, nil).Once()
	store.EXPECT().SetAccountPreferences(mock.Anything, testRealmID, testAccountID,
		mock.MatchedBy(func(v map[string]string) bool {
			return len(v) == 2 && v["ui.theme"] == "dark" && v["ui.timeFormat"] == "12"
		})).Return(nil).Once()

	require.NoError(t, uc.SetForAccount(context.Background(), testRealmID, testAccountID,
		map[string]string{"ui.theme": "dark", "ui.timeFormat": "12"}))
}

// The budget counts what the write would LEAVE, so re-saving keys already held consumes no
// new budget. Otherwise an account at the cap could not change a preference it already had.
func TestSetForAccountBudgetCountsOnlyNewKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	held := map[string]string{"ui.theme": "light"}
	for i := 0; len(held) < domain.MaxPreferencesPerAccount; i++ {
		held[fmt.Sprintf("held.%d", i)] = "x"
	}

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string(nil)).
		Return(held, nil).Once()
	store.EXPECT().SetAccountPreferences(mock.Anything, testRealmID, testAccountID,
		mock.Anything).Return(nil).Once()

	require.NoError(t, uc.SetForAccount(context.Background(), testRealmID, testAccountID,
		map[string]string{"ui.theme": "dark"}))
}

func TestSetForAccountRefusesToExceedTheBudget(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	held := map[string]string{}
	for i := 0; i < domain.MaxPreferencesPerAccount; i++ {
		held[fmt.Sprintf("held.%d", i)] = "x"
	}
	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string(nil)).
		Return(held, nil).Once()

	err := uc.SetForAccount(context.Background(), testRealmID, testAccountID,
		map[string]string{"ui.theme": "dark"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most")
	store.AssertNotCalled(t, "SetAccountPreferences")
}

// Resetting deletes the override rather than writing the default back, so the key tracks a
// later change to the workspace default instead of being pinned to today's value.
func TestResetForAccountDeletesOverrides(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	store.EXPECT().DeleteAccountPreferences(mock.Anything, testAccountID,
		[]string{"ui.theme"}).Return(nil).Once()

	require.NoError(t, uc.ResetForAccount(context.Background(), testRealmID, testAccountID,
		[]string{"ui.theme"}))
}

// A key that is not org-scoped has no workspace default: a workspace must not be able to
// dictate someone's theme.
func TestSetForOrganizationRefusesKeysThatAreNotOrgScoped(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()

	err := uc.SetForOrganization(context.Background(), testRealmID, testOrgID,
		map[string]string{"ui.theme": "dark"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "personal")
	store.AssertNotCalled(t, "SetOrganizationPreferences")
}

func TestSetForOrganizationWritesOrgScopedKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()
	store.EXPECT().SetOrganizationPreferences(mock.Anything, testRealmID, testOrgID,
		mock.MatchedBy(func(v map[string]string) bool { return v["ui.timeFormat"] == "12" })).
		Return(nil).Once()

	require.NoError(t, uc.SetForOrganization(context.Background(), testRealmID, testOrgID,
		map[string]string{"ui.timeFormat": "12"}))
}

func TestSpecsAreSorted(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	store.EXPECT().Specs(mock.Anything, testRealmID).Return(declared(), nil).Once()

	got, err := uc.Specs(context.Background(), testRealmID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "ui.theme", got[0].Key)
	assert.Equal(t, "ui.timeFormat", got[1].Key)
}
