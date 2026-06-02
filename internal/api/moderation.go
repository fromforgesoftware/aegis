package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// JSON:API types for the synthetic account-moderation operations.
const (
	ResourceTypeAccountBan   resource.Type = "accountBans"
	ResourceTypeAccountMerge resource.Type = "accountMerges"
)

// BanRequestDTO bans an account. A nil until is a permanent ban.
type BanRequestDTO struct {
	resource.RestDTO

	RAccountID string     `jsonapi:"attr,accountId"`
	RUntil     *time.Time `jsonapi:"attr,until,omitempty"`
	RReason    string     `jsonapi:"attr,reason,omitempty"`
}

// UnbanRequestDTO lifts a ban.
type UnbanRequestDTO struct {
	resource.RestDTO

	RAccountID string `jsonapi:"attr,accountId"`
}

// AccountBanDTO echoes the moderation outcome.
type AccountBanDTO struct {
	resource.RestDTO

	RAccountID string     `jsonapi:"attr,accountId"`
	RBanned    bool       `jsonapi:"attr,banned"`
	RUntil     *time.Time `jsonapi:"attr,until,omitempty"`
	RReason    string     `jsonapi:"attr,reason,omitempty"`
}

func AccountBanToDTO(accountID string, banned bool, until *time.Time, reason string) *AccountBanDTO {
	dto := &AccountBanDTO{RAccountID: accountID, RBanned: banned, RUntil: until, RReason: reason}
	dto.RType = ResourceTypeAccountBan
	dto.RID = accountID
	return dto
}

// MergeRequestDTO consolidates the source account into the target.
type MergeRequestDTO struct {
	resource.RestDTO

	RSourceID string `jsonapi:"attr,sourceId"`
	RTargetID string `jsonapi:"attr,targetId"`
}

// AccountMergeDTO reports what the merge moved onto the target.
type AccountMergeDTO struct {
	resource.RestDTO

	RSourceID    string `jsonapi:"attr,sourceId"`
	RTargetID    string `jsonapi:"attr,targetId"`
	RExternalIDs int64  `jsonapi:"attr,externalIds"`
	RMemberships int64  `jsonapi:"attr,memberships"`
	RBindings    int64  `jsonapi:"attr,bindings"`
}

func AccountMergeToDTO(sourceID, targetID string, externalIDs, memberships, bindings int64) *AccountMergeDTO {
	dto := &AccountMergeDTO{
		RSourceID: sourceID, RTargetID: targetID,
		RExternalIDs: externalIDs, RMemberships: memberships, RBindings: bindings,
	}
	dto.RType = ResourceTypeAccountMerge
	dto.RID = targetID
	return dto
}
