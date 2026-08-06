package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypePreference is the JSON:API type for /api/me/preferences.
const ResourceTypePreference resource.Type = "preferences"

// PreferenceDTO is the wire shape of one resolved preference.
//
// The JSON:API id is the KEY, not a surrogate. A preference has no identity apart
// from its key, and minting a uuid for it would give clients an identifier that
// changes when a row is deleted and rewritten — so `PATCH` bodies would have to
// carry ids the client had to look up first, to address something it already knows
// by name.
//
// `source` is part of the contract rather than an internal detail: a settings page
// has to distinguish a value the user chose from one inherited from the workspace
// or the platform default, or it cannot render "inherited" state and cannot offer
// a meaningful reset.
type PreferenceDTO struct {
	resource.RestDTO

	RKey    string `jsonapi:"attr,key"`
	RValue  string `jsonapi:"attr,value"`
	RSource string `jsonapi:"attr,source"`
}

func PreferenceToDTO(p domain.Preference) *PreferenceDTO {
	dto := &PreferenceDTO{
		RKey:    p.Key,
		RValue:  p.Value,
		RSource: string(p.Source),
	}
	dto.RType = ResourceTypePreference
	dto.RID = p.Key
	return dto
}

// PreferenceSpecDTO describes one declared key, so a client can render a settings
// page from the registry instead of duplicating every enum on its own side. The
// duplication is the thing worth avoiding: two copies of an allowed-values list
// drift, and the drift shows up as a control offering a value the API rejects.
type PreferenceSpecDTO struct {
	resource.RestDTO

	RKey string `jsonapi:"attr,key"`
	// RValueType, not RType: the embedded RestDTO already owns RType for the
	// JSON:API resource type, and a second RType here would shadow it — so
	// `dto.RType = …` would silently set the wrong field.
	RValueType string   `jsonapi:"attr,valueType"`
	RDefault   string   `jsonapi:"attr,default"`
	RAllowed   []string `jsonapi:"attr,allowed,omitempty"`
	ROrgScoped bool     `jsonapi:"attr,orgScoped"`
	RClaim     string   `jsonapi:"attr,claim,omitempty"`
}

func PreferenceSpecToDTO(s domain.PreferenceSpec) *PreferenceSpecDTO {
	dto := &PreferenceSpecDTO{
		RKey:       s.Key,
		RValueType: string(s.Type),
		RDefault:   s.Default,
		RAllowed:   s.Allowed,
		ROrgScoped: s.OrgScoped,
		RClaim:     s.Claim,
	}
	dto.RType = ResourceTypePreferenceSpec
	dto.RID = s.Key
	return dto
}

// ResourceTypePreferenceSpec is the JSON:API type for the registry listing.
const ResourceTypePreferenceSpec resource.Type = "preferenceSpecs"
