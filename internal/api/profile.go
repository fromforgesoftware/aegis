package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeProfile is the JSON:API type for /api/me.
const ResourceTypeProfile resource.Type = "profiles"

// ProfileDTO is the signed-in account's own view of itself.
//
// Deliberately narrower than an account: no realm id, no status, no login counters. This is
// what a settings form renders and what the account may edit, and exposing the moderation
// fields on a self-service endpoint would invite a client to display state it cannot act on.
//
// displayName is returned but not editable — it is DERIVED from the two name parts, so a
// client that let someone type it directly would be editing a value the next save overwrites.
type ProfileDTO struct {
	resource.RestDTO

	REmail         string `jsonapi:"attr,email"`
	REmailVerified bool   `jsonapi:"attr,emailVerified"`
	RGivenName     string `jsonapi:"attr,givenName"`
	RFamilyName    string `jsonapi:"attr,familyName"`
	RDisplayName   string `jsonapi:"attr,displayName"`
	RPhotoURL      string `jsonapi:"attr,photoUrl,omitempty"`
}

func ProfileToDTO(a domain.Account) *ProfileDTO {
	if a == nil {
		return nil
	}
	dto := &ProfileDTO{
		RestDTO:        resource.ToRestDTO(a),
		REmail:         a.Email(),
		REmailVerified: a.EmailVerified(),
		RGivenName:     a.GivenName(),
		RFamilyName:    a.FamilyName(),
		RDisplayName:   a.DisplayName(),
		RPhotoURL:      a.PhotoURL(),
	}
	dto.RType = ResourceTypeProfile
	return dto
}
