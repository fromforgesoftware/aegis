package app_test

import (
	"context"
	"strings"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

const profileAccountID = "44444444-4444-4444-8444-444444444444"

func newProfileUsecase(t *testing.T) (
	*apptest.AccountRepository, *apptest.CredentialRepository,
	*apptest.PasswordPolicyRepository, app.ProfileUsecase,
) {
	accounts := apptest.NewAccountRepository(t)
	creds := apptest.NewCredentialRepository(t)
	policies := apptest.NewPasswordPolicyRepository(t)
	return accounts, creds, policies,
		app.NewProfileUsecase(accounts, creds, policies, app.NewArgon2idHasher())
}

func existingAccount(opts ...internaltest.AccountOption) domain.Account {
	base := []internaltest.AccountOption{
		internaltest.WithAccountID(profileAccountID),
		internaltest.WithAccountRealmID("realm-1"),
		internaltest.WithAccountEmail("trader@example.com"),
	}
	return internaltest.NewAccount(append(base, opts...)...)
}

func str(s string) *string { return &s }

// --- reading ------------------------------------------------------------------------

func TestProfileReadsTheSignedInAccount(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(existingAccount(internaltest.WithAccountGivenName("Domingo")), nil).Once()

	got, err := uc.Profile(context.Background(), profileAccountID)
	require.NoError(t, err)
	assert.Equal(t, "Domingo", got.GivenName())
	assert.Equal(t, "trader@example.com", got.Email())
}

func TestProfileRequiresAnAccountID(t *testing.T) {
	_, _, _, uc := newProfileUsecase(t)
	_, err := uc.Profile(context.Background(), "")
	assert.Error(t, err)
}

// --- updating -----------------------------------------------------------------------

// The display name is derived, so the sidebar and the settings form cannot disagree about
// what someone is called.
func TestUpdateProfileDerivesTheDisplayName(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(existingAccount(internaltest.WithAccountDisplayName("old")), nil).Twice()
	accounts.EXPECT().UpdateProfileNames(mock.Anything, profileAccountID,
		"Domingo", "Sanz", "Domingo Sanz").Return(nil).Once()

	_, err := uc.UpdateProfile(context.Background(), app.UpdateProfileInput{
		AccountID: profileAccountID, GivenName: str("Domingo"), FamilyName: str("Sanz"),
	})
	require.NoError(t, err)
}

// A PATCH carrying one field must not clear the other. This is the whole reason the input
// uses pointers, and the failure it prevents is silent data loss on every partial save.
func TestUpdateProfileLeavesOmittedFieldsAlone(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(
		internaltest.WithAccountGivenName("Domingo"),
		internaltest.WithAccountFamilyName("Sanz"),
	), nil).Twice()
	// Only the given name was sent; the family name must survive.
	accounts.EXPECT().UpdateProfileNames(mock.Anything, profileAccountID,
		"Domingos", "Sanz", "Domingos Sanz").Return(nil).Once()

	_, err := uc.UpdateProfile(context.Background(), app.UpdateProfileInput{
		AccountID: profileAccountID, GivenName: str("Domingos"),
	})
	require.NoError(t, err)
}

// Clearing a field on purpose has to remain possible, which is the other half of the pointer
// distinction: an explicit empty string is a write, an absent field is not.
func TestUpdateProfileCanClearAFieldExplicitly(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(
		internaltest.WithAccountGivenName("Domingo"),
		internaltest.WithAccountFamilyName("Sanz"),
	), nil).Twice()
	accounts.EXPECT().UpdateProfileNames(mock.Anything, profileAccountID,
		"Domingo", "", "Domingo").Return(nil).Once()

	_, err := uc.UpdateProfile(context.Background(), app.UpdateProfileInput{
		AccountID: profileAccountID, FamilyName: str(""),
	})
	require.NoError(t, err)
}

// An account created before the name parts existed has a display name and nothing else.
// Deriving from two empty parts would wipe the only name on record.
func TestUpdateProfileKeepsALegacyDisplayNameWhenBothPartsAreEmpty(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(existingAccount(internaltest.WithAccountDisplayName("Sanz, Domingo")), nil).Twice()
	accounts.EXPECT().UpdateProfileNames(mock.Anything, profileAccountID,
		"", "", "Sanz, Domingo").Return(nil).Once()

	_, err := uc.UpdateProfile(context.Background(), app.UpdateProfileInput{
		AccountID: profileAccountID, GivenName: str(""), FamilyName: str(""),
	})
	require.NoError(t, err)
}

// Saving a form with no edits is a no-op, not a failure and not a write.
func TestUpdateProfileWithNothingToChangeDoesNotWrite(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()

	_, err := uc.UpdateProfile(context.Background(),
		app.UpdateProfileInput{AccountID: profileAccountID})
	require.NoError(t, err)
	accounts.AssertNotCalled(t, "UpdateProfileNames")
}

func TestUpdateProfileRejectsBadNames(t *testing.T) {
	cases := []struct {
		name  string
		given string
		want  string
	}{
		{name: "too long", given: strings.Repeat("x", 129), want: "128"},
		{name: "a control character", given: "Dom\x00ingo", want: "control character"},
		{name: "a newline", given: "Domingo\nSanz", want: "control character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			accounts, _, _, uc := newProfileUsecase(t)
			accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()

			_, err := uc.UpdateProfile(context.Background(), app.UpdateProfileInput{
				AccountID: profileAccountID, GivenName: str(tc.given),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			accounts.AssertNotCalled(t, "UpdateProfileNames")
		})
	}
}

// Names are trimmed, so a trailing space cannot produce a display name with a double gap.
func TestUpdateProfileTrimsNames(t *testing.T) {
	accounts, _, _, uc := newProfileUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Twice()
	accounts.EXPECT().UpdateProfileNames(mock.Anything, profileAccountID,
		"Domingo", "Sanz", "Domingo Sanz").Return(nil).Once()

	_, err := uc.UpdateProfile(context.Background(), app.UpdateProfileInput{
		AccountID: profileAccountID, GivenName: str("  Domingo  "), FamilyName: str(" Sanz "),
	})
	require.NoError(t, err)
}

// --- changing the password ----------------------------------------------------------

// THE reason the current password is required. An access token is enough to reach this
// endpoint, so without it a stolen token could lock the owner out of their own account —
// the one action with no self-service recovery.
func TestChangePasswordRefusesAWrongCurrentPassword(t *testing.T) {
	accounts, creds, _, uc := newProfileUsecase(t)
	hasher := app.NewArgon2idHasher()
	stored, err := hasher.Hash("theRealPassword1")
	require.NoError(t, err)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()
	creds.EXPECT().GetPasswordHash(mock.Anything, profileAccountID).
		Return(stored.Encoded, nil).Once()

	err = uc.ChangePassword(context.Background(), app.ChangePasswordInput{
		AccountID: profileAccountID, CurrentPassword: "guessing", NewPassword: "newPassword1",
	})
	require.Error(t, err)
	// An authentication failure, not a bad argument: the request was well-formed and the
	// credential wrong, which is what lets a client show the error against the right field.
	// (apierrors.Unauthorized is an alias for Unauthenticated, so that is the code it carries.)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "got %v", err)
	creds.AssertNotCalled(t, "SetPassword")
}

func TestChangePasswordSucceedsWithTheCorrectCurrentPassword(t *testing.T) {
	accounts, creds, policies, uc := newProfileUsecase(t)
	hasher := app.NewArgon2idHasher()
	stored, err := hasher.Hash("theRealPassword1")
	require.NoError(t, err)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()
	creds.EXPECT().GetPasswordHash(mock.Anything, profileAccountID).
		Return(stored.Encoded, nil).Once()
	// No realm policy configured falls back to the default, exactly as registration does.
	policies.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("passwordPolicy", "realm-1")).Once()
	creds.EXPECT().SetPassword(mock.Anything, profileAccountID, mock.Anything).Return(nil).Once()

	require.NoError(t, uc.ChangePassword(context.Background(), app.ChangePasswordInput{
		AccountID: profileAccountID, CurrentPassword: "theRealPassword1",
		NewPassword: "aDifferentOne1",
	}))
}

// The realm's policy governs a change, not just registration — otherwise the policy is only
// ever enforced on the way in and any account can downgrade itself afterwards.
func TestChangePasswordEnforcesTheRealmPolicy(t *testing.T) {
	accounts, creds, policies, uc := newProfileUsecase(t)
	hasher := app.NewArgon2idHasher()
	stored, err := hasher.Hash("theRealPassword1")
	require.NoError(t, err)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()
	creds.EXPECT().GetPasswordHash(mock.Anything, profileAccountID).
		Return(stored.Encoded, nil).Once()
	policies.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("passwordPolicy", "realm-1")).Once()

	// "short" is under the default eight-character minimum.
	err = uc.ChangePassword(context.Background(), app.ChangePasswordInput{
		AccountID: profileAccountID, CurrentPassword: "theRealPassword1", NewPassword: "short",
	})
	require.Error(t, err)
	creds.AssertNotCalled(t, "SetPassword")
}

// Re-setting the same password is a no-op the user probably did not intend, and letting it
// through would make a rotation policy trivially satisfiable.
func TestChangePasswordRefusesTheSamePassword(t *testing.T) {
	accounts, creds, _, uc := newProfileUsecase(t)
	hasher := app.NewArgon2idHasher()
	stored, err := hasher.Hash("theRealPassword1")
	require.NoError(t, err)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()
	creds.EXPECT().GetPasswordHash(mock.Anything, profileAccountID).
		Return(stored.Encoded, nil).Once()

	err = uc.ChangePassword(context.Background(), app.ChangePasswordInput{
		AccountID: profileAccountID, CurrentPassword: "theRealPassword1",
		NewPassword: "theRealPassword1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "differ")
	creds.AssertNotCalled(t, "SetPassword")
}

// A federated-only account has no password. Saying so beats reporting the current one as
// wrong, which would send the user hunting for a password they never had.
func TestChangePasswordSaysSoWhenThereIsNoPassword(t *testing.T) {
	accounts, creds, _, uc := newProfileUsecase(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existingAccount(), nil).Once()
	creds.EXPECT().GetPasswordHash(mock.Anything, profileAccountID).Return("", nil).Once()

	err := uc.ChangePassword(context.Background(), app.ChangePasswordInput{
		AccountID: profileAccountID, CurrentPassword: "x", NewPassword: "aDifferentOne1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external provider")
}

func TestChangePasswordRequiresBothPasswords(t *testing.T) {
	cases := []struct{ name, current, next string }{
		{name: "no current", current: "", next: "aDifferentOne1"},
		{name: "no new", current: "theRealPassword1", next: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			accounts, creds, _, uc := newProfileUsecase(t)
			err := uc.ChangePassword(context.Background(), app.ChangePasswordInput{
				AccountID: profileAccountID, CurrentPassword: tc.current, NewPassword: tc.next,
			})
			require.Error(t, err)
			accounts.AssertNotCalled(t, "Get")
			creds.AssertNotCalled(t, "SetPassword")
		})
	}
}
