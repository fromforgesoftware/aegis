package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestEvaluateRisk(t *testing.T) {
	p := domain.DefaultRiskPolicy() // newIP=30, newDevice=40, failure=15, stepUp=50, deny=100

	cases := []struct {
		name     string
		ctx      domain.RiskContext
		score    int
		level    domain.RiskLevel
		decision domain.RiskDecision
	}{
		{"clean login", domain.RiskContext{}, 0, domain.RiskLevelLow, domain.RiskAllow},
		{"new ip only", domain.RiskContext{NewIP: true}, 30, domain.RiskLevelLow, domain.RiskAllow},
		{"new ip + device steps up", domain.RiskContext{NewIP: true, NewDevice: true}, 70, domain.RiskLevelMedium, domain.RiskStepUp},
		{"device + failures deny", domain.RiskContext{NewDevice: true, RecentFailures: 4}, 100, domain.RiskLevelHigh, domain.RiskDeny},
		{"failures alone step up", domain.RiskContext{RecentFailures: 4}, 60, domain.RiskLevelMedium, domain.RiskStepUp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.EvaluateRisk(tc.ctx, p)
			assert.Equal(t, tc.score, got.Score)
			assert.Equal(t, tc.level, got.Level)
			assert.Equal(t, tc.decision, got.Decision)
		})
	}
}

func TestEvaluateRisk_Reasons(t *testing.T) {
	got := domain.EvaluateRisk(domain.RiskContext{NewIP: true, NewDevice: true, RecentFailures: 1}, domain.DefaultRiskPolicy())
	assert.ElementsMatch(t, []string{"new_ip", "new_device", "recent_failures"}, got.Reasons)
}
