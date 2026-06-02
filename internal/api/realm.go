package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeRealm is the JSON:API type for /api/realms.
const ResourceTypeRealm resource.Type = "realms"

// RealmDTO is the wire shape for a realm.
type RealmDTO struct {
	resource.RestDTO

	RName        string `jsonapi:"attr,name,omitempty"`
	RDisplayName string `jsonapi:"attr,displayName,omitempty"`
}

func RealmToDTO(r domain.Realm) *RealmDTO {
	if r == nil {
		return nil
	}
	dto := &RealmDTO{
		RestDTO:      resource.ToRestDTO(r),
		RName:        r.Name(),
		RDisplayName: r.DisplayName(),
	}
	dto.RType = ResourceTypeRealm
	return dto
}

func RealmFromDTO(dto *RealmDTO) domain.Realm {
	if dto == nil {
		return nil
	}
	return domain.NewRealm(dto.RName, domain.WithRealmDisplayName(dto.RDisplayName))
}
