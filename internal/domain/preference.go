package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypePreference is the resource type for account preferences.
const ResourceTypePreference resource.Type = "preferences"

// Preferences are user-scoped, product-agnostic settings: the locale a person
// reads in, the theme they prefer, which notification channels they want. They
// live in aegis because aegis owns the account, and because the alternative is
// every product in the platform storing the same person's locale separately and
// disagreeing about it.
//
// The design follows what identity providers converged on, and its one hard rule
// is that the key space is DECLARED, not free-form:
//
//   - Keycloak's declarative user profile made schema-backed attributes the
//     default and the permissive "any attribute" policy the exception, for
//     security reasons. An open key-value bag on an account is a place for
//     arbitrary caller-controlled data to accumulate.
//   - Auth0's guidance on the same feature is blunter: store only what identity
//     and access management needs, and keep detailed data in an external system.
//
// So Registry below is the whole of the key space. An unknown key is REFUSED
// rather than stored, values are validated against a declared type, and both a
// per-value and a per-account budget are enforced. What that buys, beyond not
// becoming a junk drawer, is that the settings UI can be generated from the
// registry instead of hard-coding the same enum twice on two sides of the wire.
//
// What does NOT belong here is anything product-specific — a default futures
// contract, a chart interval, a column layout. Those are scoped to a product and
// usually to a workspace too, and they belong to that product's own store. The
// dividing line is "would a second product want this same value for this same
// person": locale yes, default contract no.

// PreferenceType is the value domain of a preference.
//
// Values are stored and transported as STRINGS regardless of type. That is
// deliberate: a typed column per preference means a migration every time a
// preference is added, which is the failure mode that kills settings systems as a
// product grows. The type here is what the API validates against, so the storage
// stays uniform while the contract stays checked.
type PreferenceType string

const (
	// PreferenceTypeString is free text, bounded by MaxLen.
	PreferenceTypeString PreferenceType = "string"
	// PreferenceTypeBool is "true" or "false".
	PreferenceTypeBool PreferenceType = "bool"
	// PreferenceTypeInt is a base-10 integer.
	PreferenceTypeInt PreferenceType = "int"
	// PreferenceTypeEnum is one of Allowed.
	PreferenceTypeEnum PreferenceType = "enum"
)

// PreferenceSource says where an effective value came from.
//
// It is returned alongside the value so a settings page can show "inherited from
// the workspace" and offer a reset, rather than presenting an inherited default
// as if the user had chosen it. Without this the UI cannot tell a deliberate
// choice from a fallback, and "reset to default" becomes unimplementable.
type PreferenceSource string

const (
	// PreferenceSourceDefault means no row exists; the value is the registry's.
	PreferenceSourceDefault PreferenceSource = "default"
	// PreferenceSourceOrganization means the active organization set it.
	PreferenceSourceOrganization PreferenceSource = "organization"
	// PreferenceSourceAccount means the account overrode it.
	PreferenceSourceAccount PreferenceSource = "account"
)

// PreferenceWritePolicy is who may change a key.
type PreferenceWritePolicy string

const (
	// PreferenceWriteSelf means the account owning it may set it.
	PreferenceWriteSelf PreferenceWritePolicy = "self"
	// PreferenceWriteAdmin means only an organization administrator may set it,
	// and only at organization scope. Keycloak's guidance to prefer the strictest
	// policy per attribute is what this exists to express.
	PreferenceWriteAdmin PreferenceWritePolicy = "admin"
)

// Limits on what may be stored. Auth0 caps its equivalent feature and documents
// the caps precisely; the lesson is to have them from the start rather than
// discover the need after an account has accumulated megabytes.
const (
	// MaxPreferenceValueLen is the ceiling on any single value, whatever its
	// declared MaxLen.
	MaxPreferenceValueLen = 512
	// MaxPreferencesPerAccount bounds how many rows one account may hold. It is
	// comfortably above the registry size so the registry, not this, is the limit
	// in practice — this is the backstop if the registry ever grows carelessly.
	MaxPreferencesPerAccount = 128
)

// PreferenceSpec declares one key.
type PreferenceSpec struct {
	// Key is namespaced, dot-separated: "ui.theme", "notify.account.email". The
	// namespace is what lets a whole family move owner later without touching
	// call sites — if `ui.*` ever belongs somewhere else, it moves as a unit.
	Key string
	// Type is what Validate checks the value against.
	Type PreferenceType
	// Default is the platform-wide fallback, returned with source "default".
	Default string
	// Allowed is the permitted set for an enum, in display order.
	Allowed []string
	// MaxLen bounds a string value; zero means MaxPreferenceValueLen.
	MaxLen int
	// Write is who may set it.
	Write PreferenceWritePolicy
	// OrgScoped marks a key an organization may set a default for. A key that is
	// not org-scoped resolves from the registry straight to the account, skipping
	// the organization layer.
	OrgScoped bool
	// Claim is the OIDC standard claim this key populates, or empty.
	//
	// Only `locale` and `zoneinfo` are mapped, and only because OIDC Core section
	// 5.1 already defines them as standard claims — so a relying party knows what
	// they mean without asking us. Nothing else goes into a token: a token is
	// cached and a preference changes, so mapping `ui.theme` into one would mean
	// re-minting on every theme toggle and serving a stale theme until it expired.
	Claim string
}

// limit is the effective maximum length for this spec's values.
func (s PreferenceSpec) limit() int {
	if s.MaxLen <= 0 || s.MaxLen > MaxPreferenceValueLen {
		return MaxPreferenceValueLen
	}
	return s.MaxLen
}

// Validate reports whether a value is acceptable for this key.
func (s PreferenceSpec) Validate(value string) error {
	if len(value) > s.limit() {
		return apierrors.InvalidArgument(
			fmt.Sprintf("preference %s exceeds its %d character limit", s.Key, s.limit()))
	}
	switch s.Type {
	case PreferenceTypeBool:
		if value != "true" && value != "false" {
			return apierrors.InvalidArgument(
				fmt.Sprintf("preference %s must be \"true\" or \"false\", got %q", s.Key, value))
		}
	case PreferenceTypeInt:
		if _, err := strconv.Atoi(value); err != nil {
			return apierrors.InvalidArgument(
				fmt.Sprintf("preference %s must be an integer, got %q", s.Key, value))
		}
	case PreferenceTypeEnum:
		for _, allowed := range s.Allowed {
			if value == allowed {
				return nil
			}
		}
		return apierrors.InvalidArgument(fmt.Sprintf(
			"preference %s must be one of %s, got %q", s.Key, strings.Join(s.Allowed, ", "), value))
	case PreferenceTypeString:
		// Length, checked above, is the only constraint on free text.
	default:
		return apierrors.InvalidArgument(fmt.Sprintf("preference %s has an unknown type %q", s.Key, s.Type))
	}
	return nil
}

// The claim names this package maps, matching OIDC Core 5.1 exactly.
const (
	claimLocale   = "locale"
	claimZoneinfo = "zoneinfo"
)

// preferenceRegistry is the declared key space.
//
// Adding a key here is the only way to add a preference, and it is all that is
// required: storage is uniform, the API validates from this, and a settings page
// can render from it.
var preferenceRegistry = func() map[string]PreferenceSpec {
	specs := []PreferenceSpec{
		{
			// A BCP 47 tag. Free text rather than an enum because the set of
			// shipped locales belongs to the product's translation catalogue, not
			// to the identity service — enumerating it here would mean releasing
			// aegis to add a language.
			Key: "ui.locale", Type: PreferenceTypeString, Default: "en-GB", MaxLen: 35,
			Write: PreferenceWriteSelf, OrgScoped: true, Claim: claimLocale,
		},
		{
			// An IANA zone name. Empty means "not stated", which a client reads as
			// "use the browser's" — a guessed default would be worse than none,
			// because a wrong timezone silently shifts every displayed time.
			Key: "ui.zoneinfo", Type: PreferenceTypeString, Default: "", MaxLen: 64,
			Write: PreferenceWriteSelf, OrgScoped: true, Claim: claimZoneinfo,
		},
		{
			Key: "ui.theme", Type: PreferenceTypeEnum, Default: "auto",
			Allowed: []string{"light", "dark", "auto"}, Write: PreferenceWriteSelf,
		},
		{
			Key: "ui.fontSize", Type: PreferenceTypeEnum, Default: "default",
			Allowed: []string{"small", "default", "large"}, Write: PreferenceWriteSelf,
		},
		{
			Key: "ui.timeFormat", Type: PreferenceTypeEnum, Default: "24",
			Allowed: []string{"12", "24"}, Write: PreferenceWriteSelf, OrgScoped: true,
		},
		{
			// ISO-8601 weekday numbering: Monday is 1, Sunday is 7. Stated because
			// the alternative convention (Sunday 0) is equally common and the two
			// silently disagree by a day.
			Key: "ui.weekStart", Type: PreferenceTypeEnum, Default: "1",
			Allowed: []string{"1", "2", "3", "4", "5", "6", "7"},
			Write:   PreferenceWriteSelf, OrgScoped: true,
		},
		{
			Key: "ui.menuExpanded", Type: PreferenceTypeBool, Default: "true",
			Write: PreferenceWriteSelf,
		},
	}
	// Notification routing, one key per (category, channel). Flat keys rather than
	// a nested document because a user toggling one channel must not rewrite the
	// other eleven — that read-modify-write is exactly how two open tabs lose each
	// other's changes.
	for _, category := range []string{"account", "system"} {
		for _, channel := range []string{"email", "slack", "sms", "push"} {
			// Email defaults on: an account-security notification nobody receives
			// is worse than one nobody wanted. Every other channel is opt-in.
			def := "false"
			if channel == "email" {
				def = "true"
			}
			specs = append(specs, PreferenceSpec{
				Key:     fmt.Sprintf("notify.%s.%s", category, channel),
				Type:    PreferenceTypeBool,
				Default: def,
				Write:   PreferenceWriteSelf,
				// An organization may disable a channel platform-wide — the account
				// cannot enable what the workspace has switched off, which the
				// resolution order below enforces for admin-written keys.
				OrgScoped: true,
			})
		}
	}

	out := make(map[string]PreferenceSpec, len(specs))
	for _, s := range specs {
		out[s.Key] = s
	}
	return out
}()

// PreferenceSpecFor returns the declaration for a key.
func PreferenceSpecFor(key string) (PreferenceSpec, bool) {
	s, ok := preferenceRegistry[key]
	return s, ok
}

// PreferenceRegistry lists every declared key, ordered so a golden file and a
// generated settings page are stable.
func PreferenceRegistry() []PreferenceSpec {
	out := make([]PreferenceSpec, 0, len(preferenceRegistry))
	for _, s := range preferenceRegistry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// PreferenceClaims maps resolved preferences onto the OIDC standard claims they
// populate, skipping empty values so an unset zoneinfo does not become a claim
// asserting the empty string.
func PreferenceClaims(resolved map[string]string) map[string]string {
	claims := map[string]string{}
	for key, value := range resolved {
		spec, ok := preferenceRegistry[key]
		if !ok || spec.Claim == "" || value == "" {
			continue
		}
		claims[spec.Claim] = value
	}
	return claims
}

// Preference is one resolved preference: the effective value and where it came
// from.
type Preference struct {
	Key    string
	Value  string
	Source PreferenceSource
}

// ResolvePreferences layers the three sources into the effective set.
//
// The order is registry default, then the organization's value, then the
// account's own — each overriding the one before. Keys are taken from the
// REGISTRY rather than from the rows, so a key that has never been written still
// resolves to its default, and a row left behind by a removed key is ignored
// rather than surfaced.
//
// The one exception is an admin-written key: an organization's value wins over
// the account's, because a workspace switching off a notification channel is a
// policy the member must not be able to override. Everything else is a personal
// preference and the person's own value is final.
func ResolvePreferences(keys []string, orgValues, accountValues map[string]string) []Preference {
	if len(keys) == 0 {
		for _, spec := range PreferenceRegistry() {
			keys = append(keys, spec.Key)
		}
	}

	out := make([]Preference, 0, len(keys))
	for _, key := range keys {
		spec, ok := preferenceRegistry[key]
		if !ok {
			continue // an undeclared key has no effective value to report
		}
		p := Preference{Key: key, Value: spec.Default, Source: PreferenceSourceDefault}
		if v, ok := orgValues[key]; ok && spec.OrgScoped {
			p.Value, p.Source = v, PreferenceSourceOrganization
		}
		if v, ok := accountValues[key]; ok && spec.Write == PreferenceWriteSelf {
			p.Value, p.Source = v, PreferenceSourceAccount
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
