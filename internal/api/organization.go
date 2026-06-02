package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeOrganization is the JSON:API type for /api/organizations.
const ResourceTypeOrganization resource.Type = "organizations"

// OrganizationDTO is the wire shape for a tenant. Foreign keys are rendered as
// JSON:API relationships (realm/owner/resource) so they can be ?include=d.
type OrganizationDTO struct {
	resource.RestDTO

	RName     string         `jsonapi:"attr,name,omitempty"`
	RSlug     string         `jsonapi:"attr,slug,omitempty"`
	RStatus   string         `jsonapi:"attr,status,omitempty"`
	RSettings map[string]any `jsonapi:"attr,settings,omitempty"`
	// RRealmID is a write convenience for attribute-only clients (e.g. the
	// generic admin create form); reads expose the realm as a relationship.
	RRealmID string `jsonapi:"attr,realmId,omitempty"`

	RRealm    *resource.RelationshipDTO `jsonapi:"rel,realm,omitempty"`
	ROwner    *resource.RelationshipDTO `jsonapi:"rel,owner,omitempty"`
	RResource *resource.RelationshipDTO `jsonapi:"rel,resource,omitempty"`
}

func OrganizationToDTO(o domain.Organization) *OrganizationDTO {
	if o == nil {
		return nil
	}
	dto := &OrganizationDTO{
		RestDTO:   resource.ToRestDTO(o),
		RName:     o.Name(),
		RSlug:     o.Slug(),
		RStatus:   string(o.Status()),
		RSettings: o.Settings(),
		RRealm:    resource.RelationshipToDTO(resource.RelFromIdentifier(o.Realm())),
		ROwner:    resource.RelationshipToDTO(resource.RelFromIdentifier(o.Owner())),
		RResource: resource.RelationshipToDTO(resource.RelFromIdentifier(o.AnchorResource())),
	}
	dto.RType = ResourceTypeOrganization
	return dto
}

func OrganizationFromDTO(dto *OrganizationDTO) domain.Organization {
	if dto == nil {
		return nil
	}
	realmID := dto.RRealmID
	if dto.RRealm != nil {
		realmID = dto.RRealm.ID()
	}
	opts := []domain.OrganizationOption{}
	if dto.RStatus != "" {
		opts = append(opts, domain.WithOrganizationStatus(domain.OrgStatus(dto.RStatus)))
	}
	if dto.ROwner != nil {
		opts = append(opts, domain.WithOrganizationOwnerID(dto.ROwner.ID()))
	}
	if dto.RSettings != nil {
		opts = append(opts, domain.WithOrganizationSettings(dto.RSettings))
	}
	return domain.NewOrganization(realmID, dto.RName, dto.RSlug, opts...)
}
