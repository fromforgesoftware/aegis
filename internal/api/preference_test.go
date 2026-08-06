package api_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// This test exists because of a real bug that reached a release.
//
// The controller first rendered these DTOs with encoding/json, which knows nothing
// about `jsonapi:"attr,…"` tags — so the endpoint returned
// {"RKey":"ui.theme","RValue":"auto","RSource":"default"} instead of a JSON:API
// attributes object. It was a 200 with plausible-looking JSON, so nothing failed
// until a client tried to read `attributes.key` and found nothing there.
//
// Asserting the marshalled DOCUMENT rather than the DTO struct is the point: the
// struct was always right, and the bug lived entirely in how it was serialised.
func TestPreferenceMarshalsAsJSONAPI(t *testing.T) {
	t.Parallel()

	prefs := []domain.Preference{
		{Key: "ui.theme", Value: "dark", Source: domain.PreferenceSourceAccount},
		{Key: "ui.timeFormat", Value: "24", Source: domain.PreferenceSourceDefault},
	}

	var buf bytes.Buffer
	list := resource.ListResponseToDTO(api.PreferenceToDTO)(
		resource.NewListResponse(prefs, len(prefs)))
	require.NoError(t, jsonapi.MarshalManyPayloads(&buf, list))

	var doc struct {
		Data []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Key    string `json:"key"`
				Value  string `json:"value"`
				Source string `json:"source"`
			} `json:"attributes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	require.Len(t, doc.Data, 2)

	first := doc.Data[0]
	assert.Equal(t, "preferences", first.Type)
	// The id IS the key: a preference has no identity apart from its key, so a client
	// can address one by name without looking an opaque id up first.
	assert.Equal(t, "ui.theme", first.ID)
	assert.Equal(t, "ui.theme", first.Attributes.Key)
	assert.Equal(t, "dark", first.Attributes.Value)
	// The source is part of the contract, not an internal detail: without it a
	// settings page cannot tell a chosen value from an inherited one.
	assert.Equal(t, "account", first.Attributes.Source)

	assert.Equal(t, "default", doc.Data[1].Attributes.Source)

	// The Go field names must not appear anywhere in the payload — their presence is
	// the exact signature of the bug this guards.
	for _, leaked := range []string{"RKey", "RValue", "RSource", "RID", "RType"} {
		assert.NotContains(t, buf.String(), leaked)
	}
}

// The registry drives a generated settings page, so its enum lists and value types
// have to survive marshalling too.
func TestPreferenceSpecMarshalsAsJSONAPI(t *testing.T) {
	t.Parallel()

	specs := domain.PreferenceRegistry()
	require.NotEmpty(t, specs)

	var buf bytes.Buffer
	list := resource.ListResponseToDTO(api.PreferenceSpecToDTO)(
		resource.NewListResponse(specs, len(specs)))
	require.NoError(t, jsonapi.MarshalManyPayloads(&buf, list))

	var doc struct {
		Data []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Key       string   `json:"key"`
				ValueType string   `json:"valueType"`
				Default   string   `json:"default"`
				Allowed   []string `json:"allowed"`
				Write     string   `json:"write"`
				OrgScoped bool     `json:"orgScoped"`
				Claim     string   `json:"claim"`
			} `json:"attributes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	require.Len(t, doc.Data, len(specs))

	byKey := map[string]string{}
	for _, d := range doc.Data {
		assert.Equal(t, "preferenceSpecs", d.Type)
		assert.Equal(t, d.ID, d.Attributes.Key)
		byKey[d.Attributes.Key] = d.Attributes.ValueType
	}
	assert.Equal(t, "enum", byKey["ui.theme"])
	assert.Equal(t, "bool", byKey["notify.account.email"])
	assert.Equal(t, "string", byKey["ui.locale"])

	// The enum's allowed values must survive: a client renders its options from them,
	// and an empty list would produce a select with nothing in it.
	for _, d := range doc.Data {
		if d.Attributes.Key == "ui.theme" {
			assert.Equal(t, []string{"light", "dark", "auto"}, d.Attributes.Allowed)
		}
		// Only the two OIDC standard claims carry a claim name.
		if d.Attributes.Claim != "" {
			assert.Contains(t, []string{"locale", "zoneinfo"}, d.Attributes.Claim)
		}
	}

	// The value-type field must not collide with the JSON:API resource type. It is
	// named RValueType in Go precisely because RType would shadow RestDTO's.
	assert.NotContains(t, buf.String(), `"valueType":"preferenceSpecs"`)
}
