package api

import (
	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

const ResourceTypeRiskAssessment resource.Type = "riskAssessments"

// RiskAssessRequestDTO is the login context to score.
type RiskAssessRequestDTO struct {
	resource.RestDTO

	RRealmID   string `jsonapi:"attr,realmId,omitempty"`
	RAccountID string `jsonapi:"attr,accountId"`
	RIP        string `jsonapi:"attr,ip"`
	RDeviceID  string `jsonapi:"attr,deviceId,omitempty"`
	RSucceeded bool   `jsonapi:"attr,succeeded,omitempty"`
}

// RiskAssessmentDTO is the evaluator's verdict.
type RiskAssessmentDTO struct {
	resource.RestDTO

	RScore    int      `jsonapi:"attr,score"`
	RLevel    string   `jsonapi:"attr,level"`
	RDecision string   `jsonapi:"attr,decision"`
	RReasons  []string `jsonapi:"attr,reasons,omitempty"`
}

func RiskAssessmentToDTO(a domain.RiskAssessment) *RiskAssessmentDTO {
	dto := &RiskAssessmentDTO{
		RScore:    a.Score,
		RLevel:    string(a.Level),
		RDecision: string(a.Decision),
		RReasons:  a.Reasons,
	}
	dto.RType = ResourceTypeRiskAssessment
	return dto
}
