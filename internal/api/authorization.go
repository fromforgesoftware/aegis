package api

import (
	"encoding/json"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// JSON:API types for the synthetic (non-persisted) authorization operations.
const (
	ResourceTypeAuthorizationCheck      resource.Type = "authorizationChecks"
	ResourceTypeAuthorizationBatchCheck resource.Type = "authorizationBatchChecks"
	ResourceTypeAuthorizationVersion    resource.Type = "authorizationVersions"
)

// VersionDTO is the read-after-write counter snapshot.
type VersionDTO struct {
	resource.RestDTO

	RWriteVersion      int64 `jsonapi:"attr,writeVersion"`
	RProjectionVersion int64 `jsonapi:"attr,projectionVersion"`
}

func VersionToDTO(write, projection int64) *VersionDTO {
	dto := &VersionDTO{RWriteVersion: write, RProjectionVersion: projection}
	dto.RType = ResourceTypeAuthorizationVersion
	return dto
}

// CheckRequestDTO is the body for POST /api/authorizations/check.
type CheckRequestDTO struct {
	resource.RestDTO

	RAccountID    string `jsonapi:"attr,accountId"`
	RResourceID   string `jsonapi:"attr,resourceId"`
	RPermissionID string `jsonapi:"attr,permissionId"`
	RMinVersion   int64  `jsonapi:"attr,minVersion,omitempty"`
}

// CheckDTO is the decision response.
type CheckDTO struct {
	resource.RestDTO

	RAllowed bool `jsonapi:"attr,allowed"`
}

func CheckToDTO(allowed bool) *CheckDTO {
	dto := &CheckDTO{RAllowed: allowed}
	dto.RType = ResourceTypeAuthorizationCheck
	return dto
}

// PermissionCheckDTO is one (resource, permission) pair inside a batch.
type PermissionCheckDTO struct {
	ResourceID   string `json:"resourceId"`
	PermissionID string `json:"permissionId"`
}

// PermissionCheckList carries the batch's pairs. The kit's jsonapi decoder
// routes nested attribute structs through UnmarshalJSONAPIField, so the named
// type implements it to json-decode the array element-wise.
type PermissionCheckList []PermissionCheckDTO

func (l *PermissionCheckList) UnmarshalJSONAPIField(data []byte) error {
	return json.Unmarshal(data, (*[]PermissionCheckDTO)(l))
}

// PermissionDecisionDTO is one answer inside a batch response.
type PermissionDecisionDTO struct {
	ResourceID   string `json:"resourceId"`
	PermissionID string `json:"permissionId"`
	Allowed      bool   `json:"allowed"`
}

// BatchCheckRequestDTO is the body for POST /api/authorizations/batch-check.
type BatchCheckRequestDTO struct {
	resource.RestDTO

	RAccountID  string              `jsonapi:"attr,accountId"`
	RChecks     PermissionCheckList `jsonapi:"attr,checks"`
	RMinVersion int64               `jsonapi:"attr,minVersion,omitempty"`
}

// BatchCheckDTO carries the per-pair decisions.
type BatchCheckDTO struct {
	resource.RestDTO

	RDecisions []PermissionDecisionDTO `jsonapi:"attr,decisions"`
}

func BatchCheckToDTO(decisions []domain.PermissionDecision) *BatchCheckDTO {
	out := make([]PermissionDecisionDTO, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, PermissionDecisionDTO{
			ResourceID:   d.ResourceID,
			PermissionID: d.PermissionID,
			Allowed:      d.Allowed,
		})
	}
	dto := &BatchCheckDTO{RDecisions: out}
	dto.RType = ResourceTypeAuthorizationBatchCheck
	return dto
}

// ChecksFromDTO maps the wire pairs into domain checks.
func ChecksFromDTO(in []PermissionCheckDTO) []domain.PermissionCheck {
	out := make([]domain.PermissionCheck, 0, len(in))
	for _, c := range in {
		out = append(out, domain.PermissionCheck{ResourceID: c.ResourceID, PermissionID: c.PermissionID})
	}
	return out
}
