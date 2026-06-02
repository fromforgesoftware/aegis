package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeClient is the JSON:API type for /api/clients.
const ResourceTypeClient resource.Type = "clients"

// ClientDTO is the JSON:API representation of an OIDC client. clientSecret is
// in the create response only (raw, shown once); reads use ClientToReadDTO
// which never populates it, so a future change to Secret() can't leak.
type ClientDTO struct {
	resource.RestDTO

	RRealmID      string   `jsonapi:"attr,realmId,omitempty"`
	RClientID     string   `jsonapi:"attr,clientId,omitempty"`
	RClientType   string   `jsonapi:"attr,type,omitempty"`
	RName         string   `jsonapi:"attr,name,omitempty"`
	RGrantTypes   []string `jsonapi:"attr,grantTypes,omitempty"`
	RScopes       []string `jsonapi:"attr,scopes,omitempty"`
	RRedirectURIs []string `jsonapi:"attr,redirectUris,omitempty"`
	RPKCERequired bool     `jsonapi:"attr,pkceRequired"`
	RSecret       string   `jsonapi:"attr,clientSecret,omitempty"`
}

func clientDTO(c domain.Client) *ClientDTO {
	if c == nil {
		return nil
	}
	dto := &ClientDTO{
		RestDTO:       resource.ToRestDTO(c),
		RRealmID:      c.RealmID(),
		RClientID:     c.ClientID(),
		RClientType:   string(c.ClientType()),
		RName:         c.Name(),
		RGrantTypes:   c.GrantTypes(),
		RScopes:       c.Scopes(),
		RRedirectURIs: c.RedirectURIs(),
		RPKCERequired: c.PKCERequired(),
	}
	dto.RType = ResourceTypeClient
	return dto
}

// ClientToCreateDTO is the encoder for POST /api/clients — the only path that
// surfaces the raw clientSecret (shown exactly once).
func ClientToCreateDTO(c domain.Client) *ClientDTO {
	dto := clientDTO(c)
	if dto == nil {
		return nil
	}
	dto.RSecret = c.Secret()
	return dto
}

// ClientToReadDTO is the encoder for every non-create response. It NEVER
// populates RSecret, so the secret can't leak even if a future entity change
// makes Secret() non-empty on reads.
func ClientToReadDTO(c domain.Client) *ClientDTO {
	return clientDTO(c)
}

func ClientFromDTO(dto *ClientDTO) domain.Client {
	if dto == nil {
		return nil
	}
	return domain.NewClient(dto.RRealmID, dto.RClientID, domain.ClientType(dto.RClientType), dto.RName,
		domain.WithClientGrantTypes(dto.RGrantTypes),
		domain.WithClientScopes(dto.RScopes),
		domain.WithClientRedirectURIs(dto.RRedirectURIs),
		domain.WithClientPKCERequired(dto.RPKCERequired),
	)
}
