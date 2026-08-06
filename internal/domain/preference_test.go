package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// theme is a valid spec, used as the starting point for the invalid variants below.
func theme() domain.PreferenceSpec {
	return domain.PreferenceSpec{
		Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "auto",
		Allowed: []string{"light", "dark", "auto"},
	}
}

func doc(specs ...domain.PreferenceSpec) domain.PreferenceDocument {
	return domain.PreferenceDocument{Specs: specs}
}

// The document arrives from a values.yaml a person edits, so its validation is the only
// thing between a typo and a realm whose key space is wrong. It runs before anything is
// applied, and provisioning treats a failure as fatal.
func TestPreferenceDocumentValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		doc  domain.PreferenceDocument
		want string // substring of the expected error; empty means it must be valid
	}{
		{name: "a well-formed document", doc: doc(theme())},
		{
			name: "no specs at all",
			doc:  doc(),
			want: "declares no specs",
		},
		{
			name: "a key with no namespace",
			doc:  doc(domain.PreferenceSpec{Key: "theme", Type: domain.PreferenceTypeBool, Default: "true"}),
			want: "not namespaced",
		},
		{
			name: "a key with a space in it",
			doc: doc(domain.PreferenceSpec{
				Key: "ui.the me", Type: domain.PreferenceTypeBool, Default: "true"}),
			want: "use letters, digits and dots",
		},
		{
			name: "a capitalised segment",
			doc: doc(domain.PreferenceSpec{
				Key: "ui.Theme", Type: domain.PreferenceTypeBool, Default: "true"}),
			want: "capitalised segment",
		},
		{
			name: "the same key twice",
			doc:  doc(theme(), theme()),
			want: "declared twice",
		},
		{
			name: "an enum that allows nothing",
			doc: doc(domain.PreferenceSpec{
				Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "auto"}),
			want: "allows nothing",
		},
		{
			name: "a bool that lists allowed values",
			doc: doc(domain.PreferenceSpec{
				Key: "ui.flag", Type: domain.PreferenceTypeBool, Default: "true",
				Allowed: []string{"true", "false"}}),
			want: "lists allowed values",
		},
		{
			name: "an unknown type",
			doc:  doc(domain.PreferenceSpec{Key: "ui.thing", Type: "colour", Default: "red"}),
			want: "unknown type",
		},
		{
			// The first read of an untouched account returns the default, so a default its
			// own spec rejects would serve a value the API refuses on write.
			name: "a default the spec itself rejects",
			doc: doc(domain.PreferenceSpec{
				Key: "ui.theme", Type: domain.PreferenceTypeEnum, Default: "solarized",
				Allowed: []string{"light", "dark"}}),
			want: "illegal default",
		},
		{
			// A non-standard claim in a token is a private contract pretending to be a
			// standard one, and a cached token would serve it stale.
			name: "a claim that is not an OIDC standard one",
			doc: doc(domain.PreferenceSpec{
				Key: "ui.theme", Type: domain.PreferenceTypeString, Default: "auto",
				Claim: "theme"}),
			want: "only the OIDC standard claims",
		},
		{name: "the locale claim is allowed", doc: doc(domain.PreferenceSpec{
			Key: "ui.locale", Type: domain.PreferenceTypeString, Default: "en-GB",
			Claim: domain.ClaimLocale})},
		{name: "the zoneinfo claim is allowed", doc: doc(domain.PreferenceSpec{
			Key: "ui.zoneinfo", Type: domain.PreferenceTypeString, Default: "",
			Claim: domain.ClaimZoneinfo})},
		{
			// force authorises destroying stored values. Left in config next to a key that
			// still exists it is a standing licence to delete data that nobody notices.
			name: "force names a key that is still declared",
			doc: domain.PreferenceDocument{
				Specs: []domain.PreferenceSpec{theme()}, Force: []string{"ui.theme"}},
			want: "still declared",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.doc.Validate()
			if tc.want == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestPreferenceDocumentRefusesTooManySpecs(t *testing.T) {
	t.Parallel()

	specs := make([]domain.PreferenceSpec, 0, domain.MaxPreferenceSpecsPerRealm+1)
	for i := 0; i <= domain.MaxPreferenceSpecsPerRealm; i++ {
		specs = append(specs, domain.PreferenceSpec{
			Key:  "ui.k" + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + itoa(i),
			Type: domain.PreferenceTypeBool, Default: "true",
		})
	}
	err := doc(specs...).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func TestForcesAndKeys(t *testing.T) {
	t.Parallel()

	d := domain.PreferenceDocument{
		Specs: []domain.PreferenceSpec{
			{Key: "ui.zeta", Type: domain.PreferenceTypeBool, Default: "true"},
			{Key: "ui.alpha", Type: domain.PreferenceTypeBool, Default: "true"},
		},
		Force: []string{"ui.retired"},
	}
	// Sorted, so a golden file and a generated settings page are stable.
	assert.Equal(t, []string{"ui.alpha", "ui.zeta"}, d.Keys())
	assert.True(t, d.Forces("ui.retired"))
	assert.False(t, d.Forces("ui.alpha"))
}

func TestSpecValidate(t *testing.T) {
	t.Parallel()

	specs := map[string]domain.PreferenceSpec{
		"ui.theme":  theme(),
		"ui.locale": {Key: "ui.locale", Type: domain.PreferenceTypeString, Default: "en-GB", MaxLen: 35},
		"ui.flag":   {Key: "ui.flag", Type: domain.PreferenceTypeBool, Default: "true"},
		"ui.count":  {Key: "ui.count", Type: domain.PreferenceTypeInt, Default: "1"},
	}

	cases := []struct {
		name  string
		key   string
		value string
		ok    bool
	}{
		{name: "enum member", key: "ui.theme", value: "dark", ok: true},
		{name: "enum non-member", key: "ui.theme", value: "solarized"},
		{name: "bool true", key: "ui.flag", value: "true", ok: true},
		{name: "bool rejects 1", key: "ui.flag", value: "1"},
		{name: "bool rejects yes", key: "ui.flag", value: "yes"},
		{name: "int", key: "ui.count", value: "42", ok: true},
		{name: "int rejects text", key: "ui.count", value: "many"},
		{name: "string within its limit", key: "ui.locale", value: "en-GB", ok: true},
		{name: "string over its limit", key: "ui.locale", value: strings.Repeat("x", 36)},
		{name: "empty string is allowed", key: "ui.locale", value: "", ok: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := specs[tc.key].Validate(tc.value)
			if tc.ok {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			// The message must name the key: a settings page saving six values at once
			// needs to know which one was refused.
			assert.Contains(t, err.Error(), tc.key)
		})
	}
}

// The resolution order is the heart of the feature: spec default, then workspace, then the
// person's own choice.
func TestResolvePreferencesLayering(t *testing.T) {
	t.Parallel()

	specs := map[string]domain.PreferenceSpec{
		"ui.theme": theme(),
		"ui.timeFormat": {
			Key: "ui.timeFormat", Type: domain.PreferenceTypeEnum, Default: "24",
			Allowed: []string{"12", "24"}, OrgScoped: true,
		},
	}
	keys := []string{"ui.theme", "ui.timeFormat"}

	t.Run("nothing stored gives the spec defaults", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(specs, keys, nil, nil))
		assert.Equal(t, "auto", got["ui.theme"].Value)
		assert.Equal(t, domain.PreferenceSourceDefault, got["ui.theme"].Source)
		assert.Equal(t, "24", got["ui.timeFormat"].Value)
	})

	t.Run("the organization overrides the default", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(specs, keys,
			map[string]string{"ui.timeFormat": "12"}, nil))
		assert.Equal(t, "12", got["ui.timeFormat"].Value)
		assert.Equal(t, domain.PreferenceSourceOrganization, got["ui.timeFormat"].Source)
		// Untouched keys still report as defaults, not as inherited.
		assert.Equal(t, domain.PreferenceSourceDefault, got["ui.theme"].Source)
	})

	t.Run("the account overrides the organization", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(specs, keys,
			map[string]string{"ui.timeFormat": "12"},
			map[string]string{"ui.timeFormat": "24"}))
		assert.Equal(t, "24", got["ui.timeFormat"].Value)
		assert.Equal(t, domain.PreferenceSourceAccount, got["ui.timeFormat"].Source)
	})

	t.Run("a key that is not org-scoped ignores an organization row", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(specs, []string{"ui.theme"},
			map[string]string{"ui.theme": "light"}, nil))
		assert.Equal(t, "auto", got["ui.theme"].Value)
		assert.Equal(t, domain.PreferenceSourceDefault, got["ui.theme"].Source)
	})

	t.Run("an undeclared key resolves to nothing", func(t *testing.T) {
		t.Parallel()
		// The value tables cascade from the spec so this cannot occur in the database, but
		// resolution stays spec-first so a spec removed mid-request cannot resurface.
		got := domain.ResolvePreferences(specs, []string{"ui.retired"},
			nil, map[string]string{"ui.retired": "x"})
		assert.Empty(t, got)
	})

	t.Run("no keys means the whole declared space", func(t *testing.T) {
		t.Parallel()
		got := domain.ResolvePreferences(specs, nil, nil, nil)
		assert.Len(t, got, len(specs))
	})
}

func byKey(prefs []domain.Preference) map[string]domain.Preference {
	out := make(map[string]domain.Preference, len(prefs))
	for _, p := range prefs {
		out[p.Key] = p
	}
	return out
}
