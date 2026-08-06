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

// The read path layers all three sources and reports where each value came from.
func TestResolveLayersTheThreeSources(t *testing.T) {
	store, active, uc := newPreferenceUsecase(t)
	keys := []string{"ui.theme", "ui.timeFormat"}

	active.EXPECT().Get(mock.Anything, testAccountID).
		Return(testOrgID, "admin", true, nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, keys).
		Return(map[string]string{"ui.theme": "dark"}, nil).Once()
	store.EXPECT().OrganizationPreferences(mock.Anything, testOrgID, keys).
		Return(map[string]string{"ui.timeFormat": "12"}, nil).Once()

	got, err := uc.Resolve(context.Background(), testAccountID, keys)
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

// An account in no workspace skips the organization layer entirely — it must not
// become an error or an extra query.
func TestResolveWithNoActiveOrganization(t *testing.T) {
	store, active, uc := newPreferenceUsecase(t)

	active.EXPECT().Get(mock.Anything, testAccountID).Return("", "", false, nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string{"ui.theme"}).
		Return(map[string]string{}, nil).Once()

	got, err := uc.Resolve(context.Background(), testAccountID, []string{"ui.theme"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "auto", got[0].Value)
	assert.Equal(t, domain.PreferenceSourceDefault, got[0].Source)
}

// Preferences are read on every page load, so a fault in the unrelated active-org
// lookup must degrade to "no workspace layer" rather than break the settings page.
func TestResolveDegradesWhenTheActiveOrgLookupFails(t *testing.T) {
	store, active, uc := newPreferenceUsecase(t)

	active.EXPECT().Get(mock.Anything, testAccountID).
		Return("", "", false, errors.New("active org table unavailable")).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string{"ui.theme"}).
		Return(map[string]string{"ui.theme": "light"}, nil).Once()

	got, err := uc.Resolve(context.Background(), testAccountID, []string{"ui.theme"})
	require.NoError(t, err, "a failed org lookup must not fail the read")
	require.Len(t, got, 1)
	assert.Equal(t, "light", got[0].Value, "the account's own value still applies")
}

// A typo'd key must be refused rather than silently returning nothing, or a blank
// control looks like a backend fault.
func TestResolveRefusesUndeclaredKeys(t *testing.T) {
	_, _, uc := newPreferenceUsecase(t)

	_, err := uc.Resolve(context.Background(), testAccountID, []string{"ui.them"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ui.them")
}

func TestResolveRequiresAnAccount(t *testing.T) {
	_, _, uc := newPreferenceUsecase(t)

	_, err := uc.Resolve(context.Background(), "", nil)
	assert.Error(t, err)
}

func TestSetForAccountValidatesBeforeWriting(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name:   "undeclared key",
			values: map[string]string{"ui.wallpaper": "beach"},
			want:   "ui.wallpaper",
		},
		{
			name:   "value outside the enum",
			values: map[string]string{"ui.theme": "solarized"},
			want:   "ui.theme",
		},
		{
			name:   "non-boolean for a bool key",
			values: map[string]string{"notify.account.email": "sometimes"},
			want:   "notify.account.email",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, _, uc := newPreferenceUsecase(t)
			// No store expectation at all: nothing may reach the database. The mock
			// fails the test if an unexpected call arrives, which is the assertion.
			err := uc.SetForAccount(context.Background(), testAccountID, tc.values)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			store.AssertNotCalled(t, "SetAccountPreferences")
		})
	}
}

// The batch is all-or-nothing: one bad value must stop the good ones too, or the
// page shows some changes applied and some not with no way to tell which.
func TestSetForAccountRejectsAWholeBatchForOneBadValue(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	err := uc.SetForAccount(context.Background(), testAccountID, map[string]string{
		"ui.theme":      "dark",      // valid
		"ui.timeFormat": "half-past", // not
	})
	require.Error(t, err)
	store.AssertNotCalled(t, "SetAccountPreferences")
}

func TestSetForAccountWritesAValidBatch(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)
	values := map[string]string{"ui.theme": "dark", "ui.timeFormat": "12"}

	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string(nil)).
		Return(map[string]string{}, nil).Once()
	store.EXPECT().SetAccountPreferences(mock.Anything, testAccountID,
		mock.MatchedBy(func(v map[string]string) bool {
			return len(v) == 2 && v["ui.theme"] == "dark" && v["ui.timeFormat"] == "12"
		})).Return(nil).Once()

	require.NoError(t, uc.SetForAccount(context.Background(), testAccountID, values))
}

// The budget counts what the write would LEAVE, so re-saving keys already held
// consumes no new budget. Otherwise an account near the cap could not change a
// preference it already had.
func TestSetForAccountBudgetCountsOnlyNewKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	// Exactly AT the cap, counting ui.theme itself — so the batch below adds no new
	// key and must be allowed through.
	held := map[string]string{"ui.theme": "light"}
	for i := 0; len(held) < domain.MaxPreferencesPerAccount; i++ {
		held[fmt.Sprintf("held.%d", i)] = "x"
	}

	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string(nil)).Return(held, nil).Once()
	store.EXPECT().SetAccountPreferences(mock.Anything, testAccountID, mock.Anything).
		Return(nil).Once()

	require.NoError(t, uc.SetForAccount(context.Background(), testAccountID,
		map[string]string{"ui.theme": "dark"}))
}

func TestSetForAccountRefusesToExceedTheBudget(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	held := map[string]string{}
	for i := 0; i < domain.MaxPreferencesPerAccount; i++ {
		held[fmt.Sprintf("held.%d", i)] = "x"
	}
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID, []string(nil)).Return(held, nil).Once()

	err := uc.SetForAccount(context.Background(), testAccountID,
		map[string]string{"ui.theme": "dark"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most")
	store.AssertNotCalled(t, "SetAccountPreferences")
}

// Resetting deletes the override rather than writing the default back, so the key
// tracks a later change to the workspace default instead of being pinned.
func TestResetForAccountDeletesOverrides(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	store.EXPECT().DeleteAccountPreferences(mock.Anything, testAccountID,
		[]string{"ui.theme"}).Return(nil).Once()

	require.NoError(t, uc.ResetForAccount(context.Background(), testAccountID, []string{"ui.theme"}))
}

func TestResetForAccountRefusesUndeclaredKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	err := uc.ResetForAccount(context.Background(), testAccountID, []string{"ui.nope"})
	require.Error(t, err)
	store.AssertNotCalled(t, "DeleteAccountPreferences")
}

// A personal key has no workspace default: an organization must not be able to
// dictate someone's theme.
func TestSetForOrganizationRefusesPersonalKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	err := uc.SetForOrganization(context.Background(), testOrgID,
		map[string]string{"ui.theme": "dark"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "personal")
	store.AssertNotCalled(t, "SetOrganizationPreferences")
}

func TestSetForOrganizationWritesOrgScopedKeys(t *testing.T) {
	store, _, uc := newPreferenceUsecase(t)

	store.EXPECT().SetOrganizationPreferences(mock.Anything, testOrgID,
		mock.MatchedBy(func(v map[string]string) bool { return v["ui.timeFormat"] == "12" })).
		Return(nil).Once()

	require.NoError(t, uc.SetForOrganization(context.Background(), testOrgID,
		map[string]string{"ui.timeFormat": "12"}))
}

// Claims reads only the claim-bearing keys, because this runs on the token path
// where every extra row read is latency on every login.
func TestClaimsReadsOnlyClaimBearingKeys(t *testing.T) {
	store, active, uc := newPreferenceUsecase(t)

	active.EXPECT().Get(mock.Anything, testAccountID).Return("", "", false, nil).Once()
	store.EXPECT().AccountPreferences(mock.Anything, testAccountID,
		mock.MatchedBy(func(keys []string) bool {
			// Exactly the two OIDC standard claims, not the whole registry.
			return len(keys) == 2 &&
				((keys[0] == "ui.locale" && keys[1] == "ui.zoneinfo") ||
					(keys[0] == "ui.zoneinfo" && keys[1] == "ui.locale"))
		})).Return(map[string]string{"ui.locale": "en-IE"}, nil).Once()

	claims, err := uc.Claims(context.Background(), testAccountID)
	require.NoError(t, err)
	// zoneinfo is unset, so it is absent rather than an empty-string claim.
	assert.Equal(t, map[string]string{"locale": "en-IE"}, claims)
}
