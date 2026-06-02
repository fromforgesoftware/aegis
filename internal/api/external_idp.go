package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeExternalIDP is the JSON:API type for /api/external-idps.
const ResourceTypeExternalIDP resource.Type = "externalIdps"

// ExternalIDPDTO is the wire shape for an upstream IdP config. The encrypted
// secret is never serialized; RSecret is decoded on POST only and never set
// on read responses.
type ExternalIDPDTO struct {
	resource.RestDTO

	RRealmID      string            `jsonapi:"attr,realmId,omitempty"`
	RKind         string            `jsonapi:"attr,kind,omitempty"`
	RName         string            `jsonapi:"attr,name,omitempty"`
	REnabled      bool              `jsonapi:"attr,enabled"`
	RClientID     string            `jsonapi:"attr,clientId,omitempty"`
	RSecret       string            `jsonapi:"attr,clientSecret,omitempty"`
	RDiscoveryURL string            `jsonapi:"attr,discoveryUrl,omitempty"`
	RIssuer       string            `jsonapi:"attr,issuer,omitempty"`
	RScopes       []string          `jsonapi:"attr,scopes,omitempty"`
	RConfig       map[string]string `jsonapi:"attr,config,omitempty"`
}

// ExternalIDPToReadDTO never populates RSecret, ensuring the upstream secret
// can't leak through reads even if a future change exposes the sealed bytes.
func ExternalIDPToReadDTO(c domain.ExternalIDPConfig) *ExternalIDPDTO {
	if c == nil {
		return nil
	}
	dto := &ExternalIDPDTO{
		RestDTO:       resource.ToRestDTO(c),
		RRealmID:      c.RealmID(),
		RKind:         string(c.Kind()),
		RName:         c.Name(),
		REnabled:      c.Enabled(),
		RClientID:     c.ClientID(),
		RDiscoveryURL: c.DiscoveryURL(),
		RIssuer:       c.Issuer(),
		RScopes:       c.Scopes(),
		RConfig:       c.Config(),
	}
	dto.RType = ResourceTypeExternalIDP
	return dto
}

// ExternalIDPFromDTO decodes the request body into the domain shape; RSecret
// is read by the controller, not threaded through the domain object.
func ExternalIDPFromDTO(dto *ExternalIDPDTO) domain.ExternalIDPConfig {
	if dto == nil {
		return nil
	}
	return domain.NewExternalIDPConfig(
		dto.RRealmID, domain.ExternalIDPKind(dto.RKind), dto.RName,
		domain.WithExternalIDPEnabled(dto.REnabled),
		domain.WithExternalIDPClientID(dto.RClientID),
		domain.WithExternalIDPDiscoveryURL(dto.RDiscoveryURL),
		domain.WithExternalIDPIssuer(dto.RIssuer),
		domain.WithExternalIDPScopes(dto.RScopes),
		domain.WithExternalIDPConfig(dto.RConfig),
	)
}
