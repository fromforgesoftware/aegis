package api

import "github.com/fromforgesoftware/go-kit/resource"

// JSON:API types for the synthetic magic-link operations.
const (
	ResourceTypeMagicLink         resource.Type = "magicLinks"
	ResourceTypeMagicLinkRedeemed resource.Type = "magicLinkSessions"
)

// MagicLinkRequestDTO asks for a passwordless login link.
type MagicLinkRequestDTO struct {
	resource.RestDTO

	RRealmID string `jsonapi:"attr,realmId"`
	REmail   string `jsonapi:"attr,email"`
}

// MagicLinkAckDTO is the uniform acknowledgement (never reveals whether the
// email is registered).
type MagicLinkAckDTO struct {
	resource.RestDTO

	RRequested bool `jsonapi:"attr,requested"`
}

func MagicLinkAckToDTO() *MagicLinkAckDTO {
	dto := &MagicLinkAckDTO{RRequested: true}
	dto.RType = ResourceTypeMagicLink
	return dto
}

// MagicLinkRedeemDTO redeems a login token.
type MagicLinkRedeemDTO struct {
	resource.RestDTO

	RToken string `jsonapi:"attr,token"`
}

// MagicLinkSessionDTO identifies the account a redeemed link authenticated.
type MagicLinkSessionDTO struct {
	resource.RestDTO

	RAccountID string `jsonapi:"attr,accountId"`
	RRealmID   string `jsonapi:"attr,realmId"`
	REmail     string `jsonapi:"attr,email"`
}

func MagicLinkSessionToDTO(accountID, realmID, email string) *MagicLinkSessionDTO {
	dto := &MagicLinkSessionDTO{RAccountID: accountID, RRealmID: realmID, REmail: email}
	dto.RType = ResourceTypeMagicLinkRedeemed
	dto.RID = accountID
	return dto
}
