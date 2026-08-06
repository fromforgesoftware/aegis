package app

import (
	"context"
	"fmt"
	"sort"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// PreferenceStore persists preference rows (implemented by the db package).
type PreferenceStore interface {
	AccountPreferences(ctx context.Context, accountID string, keys []string) (map[string]string, error)
	OrganizationPreferences(ctx context.Context, orgID string, keys []string) (map[string]string, error)
	SetAccountPreferences(ctx context.Context, accountID string, values map[string]string) error
	SetOrganizationPreferences(ctx context.Context, orgID string, values map[string]string) error
	DeleteAccountPreferences(ctx context.Context, accountID string, keys []string) error
}

// PreferenceUsecase resolves and updates account preferences.
//
// Every value it accepts is validated against the registry in the domain package,
// and every value it returns is the RESOLVED effective one with its source, never
// a raw row. Callers therefore cannot receive a value for a key that has been
// retired, nor store one for a key that was never declared.
type PreferenceUsecase interface {
	// Resolve returns the effective values for keys, or for the whole registry
	// when keys is empty.
	Resolve(ctx context.Context, accountID string, keys []string) ([]domain.Preference, error)
	// SetForAccount applies a batch of account-scoped overrides.
	SetForAccount(ctx context.Context, accountID string, values map[string]string) error
	// ResetForAccount removes overrides so the keys fall back to the organization
	// or registry value.
	ResetForAccount(ctx context.Context, accountID string, keys []string) error
	// SetForOrganization applies workspace defaults. The caller is responsible for
	// having checked that the actor administers the organization; this enforces
	// only that the KEY may be set at organization scope.
	SetForOrganization(ctx context.Context, orgID string, values map[string]string) error
	// Claims returns the OIDC standard claims an account's preferences populate.
	Claims(ctx context.Context, accountID string) (map[string]string, error)
}

type preferenceUsecase struct {
	store PreferenceStore
	// active resolves the account's current organization, the middle layer of
	// resolution. It reuses the existing AccountActiveOrgRepository port rather
	// than declaring a narrower read-only one: a second interface over the same
	// store is a second name for one concept, and the repository ports here are
	// already the declared boundary.
	active AccountActiveOrgRepository
}

func NewPreferenceUsecase(store PreferenceStore, active AccountActiveOrgRepository) PreferenceUsecase {
	return &preferenceUsecase{store: store, active: active}
}

// validateKeys rejects undeclared keys before any read touches the database.
//
// Refusing rather than ignoring is the point. A client asking for `ui.them` — a
// typo — that silently received nothing would render a blank control and look like
// a backend bug; told the key does not exist, the mistake is obvious.
func validateKeys(keys []string) error {
	var unknown []string
	for _, key := range keys {
		if _, ok := domain.PreferenceSpecFor(key); !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return apierrors.InvalidArgument(fmt.Sprintf("unknown preference keys: %v", unknown))
	}
	return nil
}

func (u *preferenceUsecase) Resolve(
	ctx context.Context, accountID string, keys []string,
) ([]domain.Preference, error) {
	if accountID == "" {
		return nil, apierrors.InvalidArgument("account id is required")
	}
	if err := validateKeys(keys); err != nil {
		return nil, err
	}

	accountValues, err := u.store.AccountPreferences(ctx, accountID, keys)
	if err != nil {
		return nil, err
	}

	orgValues := map[string]string{}
	if u.active != nil {
		orgID, _, found, err := u.active.Get(ctx, accountID)
		if err != nil {
			// A failure resolving the active organization degrades to "no
			// organization layer" rather than failing the read. Preferences are
			// read on every page load; returning an error here would make an
			// unrelated fault look like a broken settings page, and the fallback
			// (registry default plus the account's own value) is still correct,
			// just not workspace-aware.
			orgValues = map[string]string{}
		} else if found {
			if orgValues, err = u.store.OrganizationPreferences(ctx, orgID, keys); err != nil {
				return nil, err
			}
		}
	}
	return domain.ResolvePreferences(keys, orgValues, accountValues), nil
}

// validateValues checks every key and value in a batch before writing any of them.
//
// All-or-nothing on purpose: a batch that wrote the valid half and reported an
// error would leave the settings page showing some changes applied and some not,
// with no way for the client to know which.
func validateValues(values map[string]string, scope domain.PreferenceSource) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if err := validateKeys(keys); err != nil {
		return err
	}

	for _, key := range keys {
		spec, _ := domain.PreferenceSpecFor(key)
		if err := spec.Validate(values[key]); err != nil {
			return err
		}
		switch scope {
		case domain.PreferenceSourceAccount:
			if spec.Write != domain.PreferenceWriteSelf {
				return apierrors.Forbidden(fmt.Sprintf(
					"preference %s is administered at the workspace level and cannot be set per account", key))
			}
		case domain.PreferenceSourceOrganization:
			if !spec.OrgScoped {
				return apierrors.InvalidArgument(fmt.Sprintf(
					"preference %s is personal and has no workspace default", key))
			}
		case domain.PreferenceSourceDefault:
			return apierrors.InvalidArgument("cannot write the registry default")
		}
	}
	return nil
}

func (u *preferenceUsecase) SetForAccount(
	ctx context.Context, accountID string, values map[string]string,
) error {
	if accountID == "" {
		return apierrors.InvalidArgument("account id is required")
	}
	if len(values) == 0 {
		return nil
	}
	if err := validateValues(values, domain.PreferenceSourceAccount); err != nil {
		return err
	}

	// The budget is checked against what the write would LEAVE, not what exists:
	// a batch of keys the account already holds consumes no new budget, so
	// counting the batch as entirely new would refuse a legitimate re-save once an
	// account neared the cap.
	held, err := u.store.AccountPreferences(ctx, accountID, nil)
	if err != nil {
		return err
	}
	after := len(held)
	for key := range values {
		if _, exists := held[key]; !exists {
			after++
		}
	}
	if after > domain.MaxPreferencesPerAccount {
		return apierrors.InvalidArgument(fmt.Sprintf(
			"an account may hold at most %d preferences; this would store %d",
			domain.MaxPreferencesPerAccount, after))
	}
	return u.store.SetAccountPreferences(ctx, accountID, values)
}

func (u *preferenceUsecase) ResetForAccount(ctx context.Context, accountID string, keys []string) error {
	if accountID == "" {
		return apierrors.InvalidArgument("account id is required")
	}
	if len(keys) == 0 {
		return nil
	}
	if err := validateKeys(keys); err != nil {
		return err
	}
	return u.store.DeleteAccountPreferences(ctx, accountID, keys)
}

func (u *preferenceUsecase) SetForOrganization(
	ctx context.Context, orgID string, values map[string]string,
) error {
	if orgID == "" {
		return apierrors.InvalidArgument("organization id is required")
	}
	if len(values) == 0 {
		return nil
	}
	if err := validateValues(values, domain.PreferenceSourceOrganization); err != nil {
		return err
	}
	return u.store.SetOrganizationPreferences(ctx, orgID, values)
}

// Claims resolves only the claim-bearing keys and maps them to their OIDC names.
//
// It reads the two keys it needs rather than the whole registry, because this runs
// on the token-issuing path where every extra row read is latency on every login.
func (u *preferenceUsecase) Claims(ctx context.Context, accountID string) (map[string]string, error) {
	var keys []string
	for _, spec := range domain.PreferenceRegistry() {
		if spec.Claim != "" {
			keys = append(keys, spec.Key)
		}
	}
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	resolved, err := u.Resolve(ctx, accountID, keys)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(resolved))
	for _, p := range resolved {
		values[p.Key] = p.Value
	}
	return domain.PreferenceClaims(values), nil
}
