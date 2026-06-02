// Package api holds Aegis's JSON:API DTOs and the domain<->DTO mappers.
// DTOs embed resource.RestDTO so every REST response is a JSON:API
// resource document; the kit's NewJsonApi* handlers marshal them.
package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeFlow is the JSON:API type for /api/auth/flows.
const ResourceTypeFlow resource.Type = "authFlows"

// FlowFieldDTO is a nested attribute (json-tagged → marshaled as a plain
// object inside attributes.requiredFields), describing one input a flow
// step expects.
type FlowFieldDTO struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}

// FlowDTO is the JSON:API representation of an interactive auth flow.
type FlowDTO struct {
	resource.RestDTO

	RRealmID         string         `jsonapi:"attr,realmId,omitempty"`
	RFlowType        string         `jsonapi:"attr,flowType,omitempty"`
	RState           string         `jsonapi:"attr,state,omitempty"`
	RExpiresAt       time.Time      `jsonapi:"attr,expiresAt,omitempty"`
	RResultAccountID string         `jsonapi:"attr,resultAccountId,omitempty"`
	RRequiredFields  []FlowFieldDTO `jsonapi:"attr,requiredFields,omitempty"`
}

func FlowToDTO(f domain.Flow) *FlowDTO {
	if f == nil {
		return nil
	}
	dto := &FlowDTO{
		RestDTO:          resource.ToRestDTO(f),
		RRealmID:         f.RealmID(),
		RFlowType:        string(f.FlowType()),
		RState:           string(f.State()),
		RExpiresAt:       f.ExpiresAt(),
		RResultAccountID: f.ResultAccountID(),
	}
	dto.RType = ResourceTypeFlow
	for _, ff := range domain.RequiredFields(f.FlowType()) {
		dto.RRequiredFields = append(dto.RRequiredFields, FlowFieldDTO{Name: ff.Name, Kind: ff.Kind, Required: ff.Required})
	}
	return dto
}

// FlowFromDTO maps a create body to a domain flow; only realmId + flowType
// are read, the rest (id, state, expiry) are server-authoritative.
func FlowFromDTO(dto *FlowDTO) domain.Flow {
	if dto == nil {
		return nil
	}
	return domain.NewFlow(dto.RRealmID, domain.FlowType(dto.RFlowType), time.Time{})
}

// FlowSubmitDTO is the write body for PATCH /api/auth/flows/{id}: the
// per-field values a flow step accepts. Typed (not a free-form map) so the
// document stays JSON:API-clean.
type FlowSubmitDTO struct {
	resource.RestDTO

	REmail       string `jsonapi:"attr,email,omitempty"`
	RPassword    string `jsonapi:"attr,password,omitempty"`
	RDisplayName string `jsonapi:"attr,displayName,omitempty"`
	RToken       string `jsonapi:"attr,token,omitempty"`
}

// Payload collapses the set fields into the flow-submission payload map the
// usecase consumes.
func (d *FlowSubmitDTO) Payload() map[string]string {
	p := map[string]string{}
	if d.REmail != "" {
		p["email"] = d.REmail
	}
	if d.RPassword != "" {
		p["password"] = d.RPassword
	}
	if d.RDisplayName != "" {
		p["displayName"] = d.RDisplayName
	}
	if d.RToken != "" {
		p["token"] = d.RToken
	}
	return p
}
