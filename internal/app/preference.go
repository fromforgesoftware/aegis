package app

import (
	"context"
	"fmt"
	"sort"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// PreferenceManagedBy is the managed_by stamp reconciliation writes on every spec it owns.
//
// One value rather than one per namespace: a realm's key space arrives as a single document,
// so a single owner is what the prune needs to scope itself, and a spec created by any other
// means is left alone by construction.
const PreferenceManagedBy = "preferences"

// PreferenceStore persists the declared key space and the values stored against it
// (implemented by the db package).
type PreferenceStore interface {
	Specs(ctx context.Context, realmID string) (map[string]domain.PreferenceSpec, error)
	UpsertSpecs(ctx context.Context, realmID, managedBy string, specs []domain.PreferenceSpec) error
	ManagedSpecKeys(ctx context.Context, realmID, managedBy string) ([]string, error)
	CountValuesForKey(ctx context.Context, realmID, key string) (int64, error)
	DeleteSpecs(ctx context.Context, realmID string, keys []string) error

	AccountPreferences(ctx context.Context, accountID string, keys []string) (map[string]string, error)
	OrganizationPreferences(ctx context.Context, orgID string, keys []string) (map[string]string, error)
	SetAccountPreferences(ctx context.Context, realmID, accountID string, values map[string]string) error
	SetOrganizationPreferences(ctx context.Context, realmID, orgID string, values map[string]string) error
	DeleteAccountPreferences(ctx context.Context, accountID string, keys []string) error
}

// PreferenceUsecase reconciles the declared key space and resolves values against it.
//
// Every value it accepts is validated against the realm's declared specs, and every value it
// returns is the RESOLVED effective one with its source, never a raw row.
type PreferenceUsecase interface {
	// Apply reconciles one realm's key space from a config document: upsert what the
	// document declares, prune what left it.
	Apply(ctx context.Context, realmID string, doc domain.PreferenceDocument) error
	// Specs is the realm's declared key space, sorted, for the registry endpoint.
	Specs(ctx context.Context, realmID string) ([]domain.PreferenceSpec, error)
	// Resolve returns the effective values for keys, or the whole key space when empty.
	Resolve(ctx context.Context, realmID, accountID string, keys []string) ([]domain.Preference, error)
	// SetForAccount applies a batch of account-scoped overrides.
	SetForAccount(ctx context.Context, realmID, accountID string, values map[string]string) error
	// ResetForAccount removes overrides so the keys fall back to the organization or spec.
	ResetForAccount(ctx context.Context, realmID, accountID string, keys []string) error
	// SetForOrganization applies workspace defaults. The caller must already have checked
	// that the actor administers the organization; this enforces only that the KEY may be
	// set at organization scope.
	SetForOrganization(ctx context.Context, realmID, orgID string, values map[string]string) error
}

type preferenceUsecase struct {
	store PreferenceStore
	// active resolves the account's current organization, the middle layer of resolution.
	// It reuses the existing AccountActiveOrgRepository port rather than declaring a
	// narrower read-only one: a second interface over the same store is a second name for
	// one concept, and the repository ports here are already the declared boundary.
	active AccountActiveOrgRepository
}

func NewPreferenceUsecase(store PreferenceStore, active AccountActiveOrgRepository) PreferenceUsecase {
	return &preferenceUsecase{store: store, active: active}
}

// Apply reconciles a realm's key space.
//
// Upsert first, then prune, in the order the catalog uses: a key that moved namespace is
// declared under its new name before the old one is considered for removal, so the window in
// which neither exists is never observable.
func (u *preferenceUsecase) Apply(
	ctx context.Context, realmID string, doc domain.PreferenceDocument,
) error {
	if realmID == "" {
		return apierrors.InvalidArgument("preference document has no realm")
	}
	if err := doc.Validate(); err != nil {
		return apierrors.InvalidArgument(err.Error())
	}
	if err := u.store.UpsertSpecs(ctx, realmID, PreferenceManagedBy, doc.Specs); err != nil {
		return err
	}
	return u.prune(ctx, realmID, doc)
}

// prune removes specs this document owns that left it — and with them, by the value tables'
// ON DELETE CASCADE, every value stored against them.
//
// Removing a key people have actually set is REFUSED unless the document forces it. That is
// the catalog's rule applied to the same kind of danger: a slimmed document must not silently
// destroy stored settings, and a mistyped key must not either. The refusal is deliberately
// loud — provisioning is fatal at startup, so a config edit that would delete data halts the
// rollout and leaves the previous pods serving the previous key space.
func (u *preferenceUsecase) prune(
	ctx context.Context, realmID string, doc domain.PreferenceDocument,
) error {
	managed, err := u.store.ManagedSpecKeys(ctx, realmID, PreferenceManagedBy)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, key := range doc.Keys() {
		keep[key] = true
	}

	var remove []string
	for _, key := range managed {
		if keep[key] {
			continue
		}
		if !doc.Forces(key) {
			n, err := u.store.CountValuesForKey(ctx, realmID, key)
			if err != nil {
				return err
			}
			if n > 0 {
				return apierrors.InvalidArgument(fmt.Sprintf(
					"cannot prune preference %q: %d stored values would be deleted "+
						"(add it to force to override)", key, n))
			}
		}
		remove = append(remove, key)
	}
	return u.store.DeleteSpecs(ctx, realmID, remove)
}

func (u *preferenceUsecase) Specs(
	ctx context.Context, realmID string,
) ([]domain.PreferenceSpec, error) {
	byKey, err := u.store.Specs(ctx, realmID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PreferenceSpec, 0, len(byKey))
	for _, spec := range byKey {
		out = append(out, spec)
	}
	// Sorted, so a generated settings page and a golden file are stable.
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// validateKeys rejects keys the realm has not declared, before any read touches the values.
//
// Refusing rather than ignoring is the point. A client asking for `ui.them` — a typo — that
// silently received nothing would render a blank control and look like a backend bug; told
// the key is not declared, the mistake is obvious.
func validateKeys(specs map[string]domain.PreferenceSpec, keys []string) error {
	var unknown []string
	for _, key := range keys {
		if _, ok := specs[key]; !ok {
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
	ctx context.Context, realmID, accountID string, keys []string,
) ([]domain.Preference, error) {
	if realmID == "" || accountID == "" {
		return nil, apierrors.InvalidArgument("realm id and account id are required")
	}
	specs, err := u.store.Specs(ctx, realmID)
	if err != nil {
		return nil, err
	}
	if err := validateKeys(specs, keys); err != nil {
		return nil, err
	}

	accountValues, err := u.store.AccountPreferences(ctx, accountID, keys)
	if err != nil {
		return nil, err
	}

	orgValues := map[string]string{}
	if u.active != nil {
		orgID, _, found, activeErr := u.active.Get(ctx, accountID)
		switch {
		case activeErr != nil:
			// A failure resolving the active organization degrades to "no organization
			// layer" rather than failing the read. Preferences are read on every page load;
			// an unrelated fault must not present as a broken settings page, and the
			// fallback — spec default plus the account's own value — is still correct.
			orgValues = map[string]string{}
		case found:
			if orgValues, err = u.store.OrganizationPreferences(ctx, orgID, keys); err != nil {
				return nil, err
			}
		}
	}
	return domain.ResolvePreferences(specs, keys, orgValues, accountValues), nil
}

// validateValues checks every key and value in a batch before writing any of them.
//
// All-or-nothing on purpose: a batch that wrote the valid half and reported an error would
// leave the settings page showing some changes applied and some not, with no way for the
// client to know which.
func validateValues(
	specs map[string]domain.PreferenceSpec, values map[string]string, scope domain.PreferenceSource,
) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if err := validateKeys(specs, keys); err != nil {
		return err
	}

	for _, key := range keys {
		spec := specs[key]
		if err := spec.Validate(values[key]); err != nil {
			return err
		}
		if scope == domain.PreferenceSourceOrganization && !spec.OrgScoped {
			return apierrors.InvalidArgument(fmt.Sprintf(
				"preference %s is personal and has no workspace default", key))
		}
	}
	return nil
}

func (u *preferenceUsecase) SetForAccount(
	ctx context.Context, realmID, accountID string, values map[string]string,
) error {
	if realmID == "" || accountID == "" {
		return apierrors.InvalidArgument("realm id and account id are required")
	}
	if len(values) == 0 {
		return nil
	}
	specs, err := u.store.Specs(ctx, realmID)
	if err != nil {
		return err
	}
	if err := validateValues(specs, values, domain.PreferenceSourceAccount); err != nil {
		return err
	}

	// The budget is checked against what the write would LEAVE, not what exists: a batch of
	// keys the account already holds consumes no new budget, so counting the batch as
	// entirely new would refuse a legitimate re-save once an account neared the cap.
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
	return u.store.SetAccountPreferences(ctx, realmID, accountID, values)
}

func (u *preferenceUsecase) ResetForAccount(
	ctx context.Context, realmID, accountID string, keys []string,
) error {
	if realmID == "" || accountID == "" {
		return apierrors.InvalidArgument("realm id and account id are required")
	}
	if len(keys) == 0 {
		return nil
	}
	specs, err := u.store.Specs(ctx, realmID)
	if err != nil {
		return err
	}
	if err := validateKeys(specs, keys); err != nil {
		return err
	}
	return u.store.DeleteAccountPreferences(ctx, accountID, keys)
}

func (u *preferenceUsecase) SetForOrganization(
	ctx context.Context, realmID, orgID string, values map[string]string,
) error {
	if realmID == "" || orgID == "" {
		return apierrors.InvalidArgument("realm id and organization id are required")
	}
	if len(values) == 0 {
		return nil
	}
	specs, err := u.store.Specs(ctx, realmID)
	if err != nil {
		return err
	}
	if err := validateValues(specs, values, domain.PreferenceSourceOrganization); err != nil {
		return err
	}
	return u.store.SetOrganizationPreferences(ctx, realmID, orgID, values)
}
