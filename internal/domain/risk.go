package domain

import "github.com/fromforgesoftware/go-kit/resource"

// RiskLevel buckets an assessment for display/telemetry.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "LOW"
	RiskLevelMedium RiskLevel = "MEDIUM"
	RiskLevelHigh   RiskLevel = "HIGH"
)

// RiskDecision is the action the auth flow should take.
type RiskDecision string

const (
	RiskAllow  RiskDecision = "ALLOW"   // proceed
	RiskStepUp RiskDecision = "STEP_UP" // require a second factor
	RiskDeny   RiskDecision = "DENY"    // block the attempt
)

// RiskPolicy weights the signals and sets the decision thresholds. A per-realm
// override is a follow-up; DefaultRiskPolicy is the floor.
type RiskPolicy struct {
	NewIPWeight     int
	NewDeviceWeight int
	FailureWeight   int
	StepUpThreshold int
	DenyThreshold   int
}

func DefaultRiskPolicy() RiskPolicy {
	return RiskPolicy{
		NewIPWeight:     30,
		NewDeviceWeight: 40,
		FailureWeight:   15,
		StepUpThreshold: 50,
		DenyThreshold:   100,
	}
}

// RiskContext is the assembled set of signals for one login attempt. The app
// layer fills it from login history; the evaluator never touches I/O.
type RiskContext struct {
	NewIP          bool
	NewDevice      bool
	RecentFailures int
}

// RiskAssessment is the evaluator's verdict.
type RiskAssessment struct {
	Score    int
	Level    RiskLevel
	Decision RiskDecision
	Reasons  []string
}

// EvaluateRisk scores a login context against a policy and decides the action.
// Pure — no time, no I/O — so it's exhaustively unit-tested. Score accumulates
// per signal; the decision is the first threshold the score crosses (deny wins
// over step-up).
func EvaluateRisk(c RiskContext, p RiskPolicy) RiskAssessment {
	score := 0
	reasons := []string{}
	if c.NewIP {
		score += p.NewIPWeight
		reasons = append(reasons, "new_ip")
	}
	if c.NewDevice {
		score += p.NewDeviceWeight
		reasons = append(reasons, "new_device")
	}
	if c.RecentFailures > 0 {
		score += c.RecentFailures * p.FailureWeight
		reasons = append(reasons, "recent_failures")
	}

	decision := RiskAllow
	switch {
	case score >= p.DenyThreshold:
		decision = RiskDeny
	case score >= p.StepUpThreshold:
		decision = RiskStepUp
	}

	return RiskAssessment{
		Score:    score,
		Level:    riskLevel(score, p),
		Decision: decision,
		Reasons:  reasons,
	}
}

func riskLevel(score int, p RiskPolicy) RiskLevel {
	switch {
	case score >= p.DenyThreshold:
		return RiskLevelHigh
	case score >= p.StepUpThreshold:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// ResourceTypeRealmRiskPolicy is the JSON:API type for a realm's risk override.
const ResourceTypeRealmRiskPolicy resource.Type = "realmRiskPolicies"

// RealmRiskPolicy is a per-realm override of the default risk weights and
// thresholds.
type RealmRiskPolicy interface {
	resource.Resource
	RealmID() string
	Policy() RiskPolicy
}

type realmRiskPolicy struct {
	resource.Resource

	realmID string
	policy  RiskPolicy
}

type RealmRiskPolicyOption func(*realmRiskPolicy)

func WithRealmRiskPolicyID(id string) RealmRiskPolicyOption {
	return func(p *realmRiskPolicy) { p.Resource = resource.Update(p.Resource, resource.WithID(id)) }
}

func NewRealmRiskPolicy(realmID string, policy RiskPolicy, opts ...RealmRiskPolicyOption) RealmRiskPolicy {
	p := &realmRiskPolicy{
		Resource: resource.New(resource.WithType(ResourceTypeRealmRiskPolicy)),
		realmID:  realmID,
		policy:   policy,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *realmRiskPolicy) RealmID() string    { return p.realmID }
func (p *realmRiskPolicy) Policy() RiskPolicy { return p.policy }
