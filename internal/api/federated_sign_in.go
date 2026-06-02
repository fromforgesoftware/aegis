package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/app"
)

// ResourceTypeFederatedSignIn is the JSON:API type for POST /api/auth/federate;
// the resource is operational (synthetic), not a persisted entity.
const ResourceTypeFederatedSignIn resource.Type = "federatedSignIns"

// FederatedSignInRequestDTO is the body shape clients post.
type FederatedSignInRequestDTO struct {
	resource.RestDTO

	RRealmID string `jsonapi:"attr,realmId"`
	RIDPName string `jsonapi:"attr,idpName"`
	RToken   string `jsonapi:"attr,token"`
}

// FederatedSignInDTO is the response: account claim + resolution flags.
type FederatedSignInDTO struct {
	resource.RestDTO

	RAccountID    string `jsonapi:"attr,accountId,omitempty"`
	REmail        string `jsonapi:"attr,email,omitempty"`
	RDisplayName  string `jsonapi:"attr,displayName,omitempty"`
	RCreated      bool   `jsonapi:"attr,created"`
	RLinkRequired bool   `jsonapi:"attr,linkRequired"`
}

func FederatedSignInToDTO(res app.ResolveAccountResult) *FederatedSignInDTO {
	dto := &FederatedSignInDTO{
		RCreated:      res.Created,
		RLinkRequired: res.LinkRequired,
	}
	dto.RType = ResourceTypeFederatedSignIn
	if res.Account != nil {
		dto.RAccountID = res.Account.ID()
		dto.REmail = res.Account.Email()
		dto.RDisplayName = res.Account.DisplayName()
		dto.RestDTO = resource.ToRestDTO(res.Account)
		dto.RType = ResourceTypeFederatedSignIn
	}
	return dto
}
