package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// The registry is effectively a compile-time constant and the whole key space, so
// a malformed entry would ship a preference nobody can set.
func TestRegistryIsWellFormed(t *testing.T) {
	t.Parallel()

	specs := domain.PreferenceRegistry()
	require.NotEmpty(t, specs)

	seen := map[string]bool{}
	for i, s := range specs {
		assert.NotEmpty(t, s.Key, "spec %d has no key", i)
		assert.False(t, seen[s.Key], "duplicate key %s", s.Key)
		seen[s.Key] = true

		// Namespaced, so a family can move owner as a unit later.
		assert.Contains(t, s.Key, ".", "key %s is not namespaced", s.Key)
		assert.NotEmpty(t, s.Write, "key %s declares no write policy", s.Key)

		// Every default must itself be a legal value, or the very first read of an
		// untouched account returns something the API would refuse on write.
		assert.NoError(t, s.Validate(s.Default), "key %s has an illegal default", s.Key)

		if s.Type == domain.PreferenceTypeEnum {
			assert.NotEmpty(t, s.Allowed, "enum %s allows nothing", s.Key)
		}
		// Sorted, so a golden file and a generated settings page are stable.
		if i > 0 {
			assert.Less(t, specs[i-1].Key, s.Key)
		}
	}
}

// Only the two OIDC standard claims may be mapped. Anything else in a token would
// be stale the moment the user changed it, and would need a re-mint to fix.
func TestOnlyStandardClaimsAreMapped(t *testing.T) {
	t.Parallel()

	mapped := map[string]string{}
	for _, s := range domain.PreferenceRegistry() {
		if s.Claim != "" {
			mapped[s.Key] = s.Claim
		}
	}
	assert.Equal(t, map[string]string{
		"ui.locale":   "locale",
		"ui.zoneinfo": "zoneinfo",
	}, mapped)
}

func TestPreferenceClaimsSkipsEmptyValues(t *testing.T) {
	t.Parallel()

	// An unset zoneinfo must not become a claim asserting the empty string — a
	// relying party would read that as "this user is in UTC" rather than "unknown".
	claims := domain.PreferenceClaims(map[string]string{
		"ui.locale":   "en-IE",
		"ui.zoneinfo": "",
		"ui.theme":    "dark",
	})
	assert.Equal(t, map[string]string{"locale": "en-IE"}, claims)
}

func TestSpecValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		key   string
		value string
		ok    bool
	}{
		{name: "enum member", key: "ui.theme", value: "dark", ok: true},
		{name: "enum non-member", key: "ui.theme", value: "solarized"},
		{name: "bool true", key: "ui.menuExpanded", value: "true", ok: true},
		{name: "bool false", key: "ui.menuExpanded", value: "false", ok: true},
		{name: "bool rejects 1", key: "ui.menuExpanded", value: "1"},
		{name: "bool rejects yes", key: "notify.account.email", value: "yes"},
		{name: "locale string", key: "ui.locale", value: "en-GB", ok: true},
		{name: "locale over its limit", key: "ui.locale", value: strings.Repeat("x", 36)},
		{name: "weekStart in range", key: "ui.weekStart", value: "7", ok: true},
		{name: "weekStart out of range", key: "ui.weekStart", value: "8"},
		{name: "empty zoneinfo is allowed", key: "ui.zoneinfo", value: "", ok: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, found := domain.PreferenceSpecFor(tc.key)
			require.True(t, found, "%s must be declared", tc.key)

			err := spec.Validate(tc.value)
			if tc.ok {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			// The message must name the key: a settings page saving six values at
			// once needs to know which one was refused.
			assert.Contains(t, err.Error(), tc.key)
		})
	}
}

// The resolution order is the heart of the feature: default, then workspace, then
// the person's own choice.
func TestResolvePreferencesLayering(t *testing.T) {
	t.Parallel()

	keys := []string{"ui.theme", "ui.timeFormat", "ui.locale"}

	t.Run("nothing stored gives registry defaults", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(keys, nil, nil))
		assert.Equal(t, "auto", got["ui.theme"].Value)
		assert.Equal(t, domain.PreferenceSourceDefault, got["ui.theme"].Source)
		assert.Equal(t, "24", got["ui.timeFormat"].Value)
		assert.Equal(t, "en-GB", got["ui.locale"].Value)
	})

	t.Run("the organization overrides the default", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(keys,
			map[string]string{"ui.timeFormat": "12", "ui.locale": "en-IE"}, nil))
		assert.Equal(t, "12", got["ui.timeFormat"].Value)
		assert.Equal(t, domain.PreferenceSourceOrganization, got["ui.timeFormat"].Source)
		assert.Equal(t, "en-IE", got["ui.locale"].Value)
		// Untouched keys still report as defaults, not as inherited.
		assert.Equal(t, domain.PreferenceSourceDefault, got["ui.theme"].Source)
	})

	t.Run("the account overrides the organization", func(t *testing.T) {
		t.Parallel()
		got := byKey(domain.ResolvePreferences(keys,
			map[string]string{"ui.timeFormat": "12"},
			map[string]string{"ui.timeFormat": "24", "ui.theme": "dark"}))
		assert.Equal(t, "24", got["ui.timeFormat"].Value)
		assert.Equal(t, domain.PreferenceSourceAccount, got["ui.timeFormat"].Source)
		assert.Equal(t, "dark", got["ui.theme"].Value)
		assert.Equal(t, domain.PreferenceSourceAccount, got["ui.theme"].Source)
	})

	t.Run("a personal key ignores an organization row", func(t *testing.T) {
		t.Parallel()
		// ui.theme is not org-scoped, so a row at that scope — however it got
		// there — must not leak into the effective value.
		got := byKey(domain.ResolvePreferences([]string{"ui.theme"},
			map[string]string{"ui.theme": "light"}, nil))
		assert.Equal(t, "auto", got["ui.theme"].Value)
		assert.Equal(t, domain.PreferenceSourceDefault, got["ui.theme"].Source)
	})

	t.Run("an undeclared key resolves to nothing", func(t *testing.T) {
		t.Parallel()
		// A row left behind by a retired key must not surface as a preference.
		got := domain.ResolvePreferences([]string{"ui.retired"},
			nil, map[string]string{"ui.retired": "x"})
		assert.Empty(t, got)
	})

	t.Run("no keys means the whole registry", func(t *testing.T) {
		t.Parallel()
		got := domain.ResolvePreferences(nil, nil, nil)
		assert.Len(t, got, len(domain.PreferenceRegistry()))
	})
}

// Email defaults on, every other channel off: a security notification nobody
// receives is worse than one nobody asked for, and the reverse for the rest.
func TestNotificationDefaults(t *testing.T) {
	t.Parallel()

	got := byKey(domain.ResolvePreferences(nil, nil, nil))
	for _, category := range []string{"account", "system"} {
		assert.Equal(t, "true", got["notify."+category+".email"].Value, category+" email")
		for _, channel := range []string{"slack", "sms", "push"} {
			assert.Equal(t, "false", got["notify."+category+"."+channel].Value, category+" "+channel)
		}
	}
}

func byKey(prefs []domain.Preference) map[string]domain.Preference {
	out := make(map[string]domain.Preference, len(prefs))
	for _, p := range prefs {
		out[p.Key] = p
	}
	return out
}
