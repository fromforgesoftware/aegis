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

// Preferences are user-scoped settings — the locale someone reads in, the theme they
// prefer, which notification channels they want. They live in aegis because aegis owns
// the account, and because the alternative is every product storing the same person's
// locale separately and disagreeing about it.
//
// The KEY SPACE IS DATA, declared per realm and reconciled from a config document. It is
// deliberately not a list in this file: a key space compiled into the identity service
// means a code change, two releases and a redeploy to add one setting, and it forces every
// product to share one set of keys when products do not share their settings. Declaring it
// per realm in each product's values.yaml makes adding a preference a configuration change.
//
// This mirrors the authz catalog exactly — same ConfigMap-and-checksum delivery, same
// managed_by ownership stamp, same gated prune — because it is the same problem: product
// vocabulary that the platform stores but does not define.
//
// The discipline a declared space still buys is what Keycloak and Auth0 both landed on:
// Keycloak made schema-backed user attributes the default and "any attribute" the
// exception, for security reasons, and Auth0's guidance is to store only what identity
// needs. Declaring the vocabulary per product keeps that discipline without hardcoding it.

// PreferenceType is the value domain of a preference.
//
// Values are stored and transported as STRINGS whatever their type. A typed column per
// preference would mean a migration per preference, which is the failure this design
// exists to avoid. The type is what the API validates against, so storage stays uniform
// while the contract stays checked.
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
// Returned alongside the value so a settings page can show "inherited from the workspace"
// and offer a reset, rather than presenting a fallback as a choice the user made. Without
// it the UI cannot tell the two apart and "reset to default" is unimplementable.
type PreferenceSource string

const (
	// PreferenceSourceDefault means no row exists; the value is the spec's.
	PreferenceSourceDefault PreferenceSource = "default"
	// PreferenceSourceOrganization means the active organization set it.
	PreferenceSourceOrganization PreferenceSource = "organization"
	// PreferenceSourceAccount means the account overrode it.
	PreferenceSourceAccount PreferenceSource = "account"
)

// Limits on what may be stored. Auth0 caps its equivalent feature and documents the caps
// precisely; the lesson is to have them from the start rather than discover the need once
// an account has accumulated megabytes.
const (
	// MaxPreferenceValueLen is the ceiling on any single value, whatever its declared
	// MaxLen.
	MaxPreferenceValueLen = 512
	// MaxPreferencesPerAccount bounds how many rows one account may hold.
	MaxPreferencesPerAccount = 128
	// MaxPreferenceSpecsPerRealm bounds a config document, so a malformed values file
	// cannot ask aegis to declare an unbounded key space.
	MaxPreferenceSpecsPerRealm = 256
	// maxPreferenceKeyLen bounds a key, which travels in a query string.
	maxPreferenceKeyLen = 128
)

// PreferenceSpec declares one key.
//
// The json tags are the config contract: this struct is what a product writes under
// `aegis.preferences.specs` in its values.yaml, so renaming a field is a breaking change
// to every product's configuration.
type PreferenceSpec struct {
	// Key is namespaced and dot-separated: "ui.theme", "notify.account.email",
	// "trading.chartInterval". The namespace is what lets a family move owner later
	// without touching call sites.
	Key string `json:"key"`
	// Type is what Validate checks values against.
	Type PreferenceType `json:"type"`
	// Default is the fallback, returned with source "default".
	Default string `json:"default"`
	// Allowed is the permitted set for an enum, in display order.
	Allowed []string `json:"allowed,omitempty"`
	// MaxLen bounds a string value; zero means MaxPreferenceValueLen.
	MaxLen int `json:"maxLen,omitempty"`
	// OrgScoped marks a key an organization may set a default for. A key that is not
	// org-scoped resolves from the spec's default straight to the account's override.
	OrgScoped bool `json:"orgScoped,omitempty"`
	// Claim is the OIDC standard claim this key populates, or empty.
	//
	// Only `locale` and `zoneinfo` are legitimate, because OIDC Core 5.1 already defines
	// them — a relying party knows what they mean without asking. Validate refuses
	// anything else: a non-standard claim in a token is a private contract pretending to
	// be a standard one, and a token is cached, so a preference mapped into one serves a
	// stale value until it expires.
	Claim string `json:"claim,omitempty"`
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
		return apierrors.InvalidArgument(
			fmt.Sprintf("preference %s has an unknown type %q", s.Key, s.Type))
	}
	return nil
}

// The only claim names a spec may map, matching OIDC Core 5.1.
const (
	ClaimLocale   = "locale"
	ClaimZoneinfo = "zoneinfo"
)

// validKey is the key grammar: dot-separated segments of letters and digits, each starting
// lowercase. Constraining it keeps keys legible in a URL query string and stops a config
// typo from declaring something like "ui.theme " that no client can ask for without
// guessing the whitespace.
func validKey(key string) error {
	if key == "" {
		return fmt.Errorf("a preference needs a key")
	}
	if len(key) > maxPreferenceKeyLen {
		return fmt.Errorf("preference key %q is longer than %d characters", key, maxPreferenceKeyLen)
	}
	segments := strings.Split(key, ".")
	if len(segments) < 2 {
		return fmt.Errorf("preference key %q is not namespaced (expected something like ui.theme)", key)
	}
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("preference key %q has an empty segment", key)
		}
		for _, r := range seg {
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			isDigit := r >= '0' && r <= '9'
			if !isLetter && !isDigit {
				return fmt.Errorf("preference key %q contains %q; use letters, digits and dots", key, r)
			}
		}
		if first := rune(seg[0]); first >= 'A' && first <= 'Z' {
			return fmt.Errorf("preference key %q has a capitalised segment %q", key, seg)
		}
	}
	return nil
}

// PreferenceDocument is one realm's declared key space, as it arrives from config.
//
// It carries Force for the same reason CatalogDocument does: removing a spec destroys
// every value stored against it, so reconciliation refuses to prune a key accounts have
// actually set unless the document says to. A slimmed document must not silently delete
// people's settings, and neither must a typo.
type PreferenceDocument struct {
	// Realm names the realm this key space belongs to. Empty means the server's
	// bootstrap realm, which is the single-product case and the common one.
	Realm string `json:"realm,omitempty"`
	// Force lists keys whose stored values may be destroyed when the key leaves the
	// document. Anything not listed is protected by the prune gate.
	Force []string `json:"force,omitempty"`
	// Specs is the key space.
	Specs []PreferenceSpec `json:"specs"`
}

// Forces reports whether the document authorises destroying this key's stored values.
func (d PreferenceDocument) Forces(key string) bool {
	for _, f := range d.Force {
		if f == key {
			return true
		}
	}
	return false
}

// Validate checks the whole document before any of it is applied.
//
// All-or-nothing, and fatal at startup: provisioning treats an invalid document the way
// the catalog does — fail readiness and halt the rollout, so the previous pods keep
// serving the previous key space rather than half of a new one.
func (d PreferenceDocument) Validate() error {
	if len(d.Specs) == 0 {
		return fmt.Errorf("preference document declares no specs")
	}
	if len(d.Specs) > MaxPreferenceSpecsPerRealm {
		return fmt.Errorf("preference document declares %d specs, at most %d are allowed",
			len(d.Specs), MaxPreferenceSpecsPerRealm)
	}

	seen := map[string]bool{}
	for _, s := range d.Specs {
		if err := validKey(s.Key); err != nil {
			return err
		}
		if seen[s.Key] {
			return fmt.Errorf("preference key %q is declared twice", s.Key)
		}
		seen[s.Key] = true

		switch s.Type {
		case PreferenceTypeString, PreferenceTypeBool, PreferenceTypeInt:
			if len(s.Allowed) > 0 {
				return fmt.Errorf("preference %s is a %s but lists allowed values", s.Key, s.Type)
			}
		case PreferenceTypeEnum:
			if len(s.Allowed) == 0 {
				return fmt.Errorf("preference %s is an enum but allows nothing", s.Key)
			}
		default:
			return fmt.Errorf(
				"preference %s has an unknown type %q (want string, bool, int or enum)", s.Key, s.Type)
		}

		// A default its own spec would reject means the very first read of an untouched
		// account returns a value the API refuses on write.
		if err := s.Validate(s.Default); err != nil {
			return fmt.Errorf("preference %s has an illegal default: %w", s.Key, err)
		}

		if s.Claim != "" && s.Claim != ClaimLocale && s.Claim != ClaimZoneinfo {
			return fmt.Errorf(
				"preference %s maps claim %q; only the OIDC standard claims %q and %q may be mapped",
				s.Key, s.Claim, ClaimLocale, ClaimZoneinfo)
		}
	}

	// A force entry naming a key still in the document is either a leftover from a
	// completed prune or a misunderstanding of what force does. Either way it is a
	// standing authorisation to destroy data sitting unnoticed in config.
	for _, f := range d.Force {
		if seen[f] {
			return fmt.Errorf("force lists %q, which is still declared; remove one of the two", f)
		}
	}
	return nil
}

// SpecsByKey indexes the document for resolution.
func (d PreferenceDocument) SpecsByKey() map[string]PreferenceSpec {
	out := make(map[string]PreferenceSpec, len(d.Specs))
	for _, s := range d.Specs {
		out[s.Key] = s
	}
	return out
}

// Keys lists the document's keys, sorted.
func (d PreferenceDocument) Keys() []string {
	out := make([]string, 0, len(d.Specs))
	for _, s := range d.Specs {
		out = append(out, s.Key)
	}
	sort.Strings(out)
	return out
}

// Preference is one resolved preference: the effective value and where it came from.
type Preference struct {
	Key    string
	Value  string
	Source PreferenceSource
}

// ResolvePreferences layers the three sources into the effective set.
//
// Spec default, then the organization's value, then the account's own — each overriding
// the one before. The keys come from the SPECS rather than from the stored rows, so a key
// nobody has written still resolves to its default.
//
// A stored value whose key has no spec cannot occur any more: the value tables reference
// preference_spec with ON DELETE CASCADE, so the database will not hold one. The lookup is
// still spec-first, which keeps the invariant true even for a spec removed between the
// read and the resolve.
func ResolvePreferences(
	specs map[string]PreferenceSpec, keys []string, orgValues, accountValues map[string]string,
) []Preference {
	if len(keys) == 0 {
		keys = make([]string, 0, len(specs))
		for key := range specs {
			keys = append(keys, key)
		}
	}

	out := make([]Preference, 0, len(keys))
	for _, key := range keys {
		spec, ok := specs[key]
		if !ok {
			continue // an undeclared key has no effective value to report
		}
		p := Preference{Key: key, Value: spec.Default, Source: PreferenceSourceDefault}
		if v, ok := orgValues[key]; ok && spec.OrgScoped {
			p.Value, p.Source = v, PreferenceSourceOrganization
		}
		if v, ok := accountValues[key]; ok {
			p.Value, p.Source = v, PreferenceSourceAccount
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
