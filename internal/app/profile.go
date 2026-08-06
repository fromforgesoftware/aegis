package app

import (
	"context"
	"strings"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// Profile is the self-service surface: the signed-in account reading and editing its own
// details.
//
// It is a separate usecase from AuthxUsecase because the authorisation model is different in
// kind. Register and Login act on a credential presented by someone who is not yet known;
// everything here acts on the account the request is already authenticated as, and the
// transport layer establishes that identity before any of it is called. Folding "change my
// own password" into the same usecase as "log in" would put those two rules in one place and
// invite the wrong one being applied.
type ProfileUsecase interface {
	// Profile reads the signed-in account.
	Profile(ctx context.Context, accountID string) (domain.Account, error)
	// UpdateProfile changes the editable parts of the profile. Email is not among them: a
	// changed address has to be re-verified before anything may be sent to it, which is a
	// flow rather than a field.
	UpdateProfile(ctx context.Context, in UpdateProfileInput) (domain.Account, error)
	// ChangePassword replaces the password, verifying the current one first.
	ChangePassword(ctx context.Context, in ChangePasswordInput) error
}

// UpdateProfileInput is the editable profile.
//
// The name fields are pointers so "leave this alone" is expressible separately from "set this
// to empty". Without that distinction a PATCH omitting a field would clear it, which makes
// every partial update destructive.
type UpdateProfileInput struct {
	AccountID  string
	GivenName  *string
	FamilyName *string
}

// ChangePasswordInput carries both passwords.
//
// CurrentPassword is required, and that is a deliberate departure from the sibling product's
// equivalent screen, which asks only for the new one. An access token is enough to reach this
// endpoint, so without the current password a stolen or borrowed token could lock the real
// owner out of their own account — the one action from which there is no self-service
// recovery. Re-authenticating here costs the user one field.
type ChangePasswordInput struct {
	AccountID       string
	CurrentPassword string
	NewPassword     string
}

type profileUsecase struct {
	accounts AccountRepository
	creds    CredentialRepository
	policies PasswordPolicyRepository
	hasher   PasswordHasher
}

func NewProfileUsecase(
	accounts AccountRepository,
	creds CredentialRepository,
	policies PasswordPolicyRepository,
	hasher PasswordHasher,
) ProfileUsecase {
	return &profileUsecase{accounts: accounts, creds: creds, policies: policies, hasher: hasher}
}

func (uc *profileUsecase) Profile(ctx context.Context, accountID string) (domain.Account, error) {
	if accountID == "" {
		return nil, apierrors.InvalidArgument("account id is required")
	}
	acc, err := uc.accounts.Get(ctx,
		search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, accountID)))
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, apierrors.NotFound("account", accountID)
	}
	return acc, nil
}

// maxNamePartLen bounds a name field. Long enough for any real name, short enough that the
// column is not a place to store a paragraph.
const maxNamePartLen = 128

func (uc *profileUsecase) UpdateProfile(
	ctx context.Context, in UpdateProfileInput,
) (domain.Account, error) {
	acc, err := uc.Profile(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}
	if in.GivenName == nil && in.FamilyName == nil {
		// Nothing to change is not an error — a form saved without edits should be a no-op
		// rather than a failure — but neither is it a write.
		return acc, nil
	}

	given, family := acc.GivenName(), acc.FamilyName()
	if in.GivenName != nil {
		if given, err = cleanNamePart(*in.GivenName, "givenName"); err != nil {
			return nil, err
		}
	}
	if in.FamilyName != nil {
		if family, err = cleanNamePart(*in.FamilyName, "familyName"); err != nil {
			return nil, err
		}
	}

	// The display name is DERIVED whenever the parts are known, so the menu and the settings
	// form cannot disagree about what someone is called. It is only left alone when both parts
	// are empty, which is the pre-migration account that has a display name and no parts —
	// clearing that would lose the only name on record.
	display := acc.DisplayName()
	if joined := strings.TrimSpace(given + " " + family); joined != "" {
		display = joined
	}

	if err := uc.accounts.UpdateProfileNames(ctx, acc.ID(), given, family, display); err != nil {
		return nil, err
	}
	// Re-read rather than returning a locally assembled account: the caller renders what was
	// stored, and constructing the response by hand is how a response starts claiming a write
	// that did not land.
	return uc.Profile(ctx, acc.ID())
}

// cleanNamePart trims and bounds one name field.
func cleanNamePart(value, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > maxNamePartLen {
		return "", apierrors.InvalidArgument(field + " is longer than 128 characters")
	}
	// Control characters in a name reach a UI as broken markup or an injected line break in an
	// email header, and no real name contains one.
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return "", apierrors.InvalidArgument(field + " contains a control character")
		}
	}
	return trimmed, nil
}

func (uc *profileUsecase) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	if in.AccountID == "" {
		return apierrors.InvalidArgument("account id is required")
	}
	if in.CurrentPassword == "" {
		return apierrors.InvalidArgument("the current password is required")
	}
	if in.NewPassword == "" {
		return apierrors.InvalidArgument("the new password is required")
	}

	acc, err := uc.Profile(ctx, in.AccountID)
	if err != nil {
		return err
	}

	encoded, err := uc.creds.GetPasswordHash(ctx, acc.ID())
	if err != nil {
		return err
	}
	if encoded == "" {
		// An account authenticated only through a federated provider has no password to
		// change, and saying so is more useful than reporting the current one as wrong.
		return apierrors.InvalidArgument(
			"this account has no password; it signs in through an external provider")
	}
	ok, err := uc.hasher.Verify(in.CurrentPassword, encoded)
	if err != nil {
		return err
	}
	if !ok {
		// Unauthorized rather than InvalidArgument: the request was well-formed and the
		// credential was wrong, which is what a client needs to distinguish to show the error
		// against the right field.
		return apierrors.Unauthorized("the current password is incorrect")
	}
	if in.CurrentPassword == in.NewPassword {
		return apierrors.InvalidArgument("the new password must differ from the current one")
	}

	// The realm's own policy, through the helper every other password-setting flow uses: the
	// same minimum length and character classes that governed registration must govern a
	// change, or the policy is only ever enforced on the way in.
	if err := validatePassword(ctx, uc.policies, acc.RealmID(), in.NewPassword); err != nil {
		return err
	}

	hashed, err := uc.hasher.Hash(in.NewPassword)
	if err != nil {
		return err
	}
	return uc.creds.SetPassword(ctx, acc.ID(), hashed)
}
