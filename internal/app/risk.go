package app

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// failureWindow bounds how far back recent-failure counting looks.
const failureWindow = 15 * time.Minute

// LoginSignal is one recorded login attempt's context.
type LoginSignal struct {
	AccountID string
	RealmID   string
	IP        string
	DeviceID  string
	Succeeded bool
}

// LoginSignalRepository records login attempts and answers the history queries
// the risk evaluator needs.
type LoginSignalRepository interface {
	Record(ctx context.Context, s LoginSignal) error
	SeenIP(ctx context.Context, accountID, ip string) (bool, error)
	SeenDevice(ctx context.Context, accountID, deviceID string) (bool, error)
	RecentFailures(ctx context.Context, accountID string, since time.Time) (int, error)
}

// RealmRiskPolicyRepository persists per-realm overrides of the risk policy.
type RealmRiskPolicyRepository interface {
	Upsert(ctx context.Context, p domain.RealmRiskPolicy) (domain.RealmRiskPolicy, error)
	GetByRealm(ctx context.Context, realmID string) (domain.RealmRiskPolicy, error)
}

// RiskPolicyUsecase is the admin surface for per-realm risk tuning.
type RiskPolicyUsecase interface {
	SetPolicy(ctx context.Context, realmID string, policy domain.RiskPolicy) (domain.RealmRiskPolicy, error)
	GetPolicy(ctx context.Context, realmID string) (domain.RiskPolicy, error)
}

type riskPolicyUsecase struct {
	policies RealmRiskPolicyRepository
}

func NewRiskPolicyUsecase(policies RealmRiskPolicyRepository) RiskPolicyUsecase {
	return &riskPolicyUsecase{policies: policies}
}

func (uc *riskPolicyUsecase) SetPolicy(ctx context.Context, realmID string, policy domain.RiskPolicy) (domain.RealmRiskPolicy, error) {
	if realmID == "" {
		return nil, apierrors.InvalidArgument("realm_id is required")
	}
	return uc.policies.Upsert(ctx, domain.NewRealmRiskPolicy(realmID, policy))
}

// GetPolicy returns the realm's override, or the service default when none is set.
func (uc *riskPolicyUsecase) GetPolicy(ctx context.Context, realmID string) (domain.RiskPolicy, error) {
	p, err := uc.policies.GetByRealm(ctx, realmID)
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return domain.DefaultRiskPolicy(), nil
		}
		return domain.RiskPolicy{}, err
	}
	return p.Policy(), nil
}

// RiskInput is the context of a login attempt to assess.
type RiskInput struct {
	RealmID   string
	AccountID string
	IP        string
	DeviceID  string
	Succeeded bool
}

// RiskUsecase scores a login attempt and records it. It sources signals from
// login history, evaluates them with the pure domain evaluator, then records
// the attempt — so the current attempt never counts itself as "seen". Wiring
// the decision into the core login flow is a follow-up; this is the standalone
// assessment surface a flow/gateway calls.
type RiskUsecase interface {
	Assess(ctx context.Context, in RiskInput) (domain.RiskAssessment, error)
}

type riskUsecase struct {
	signals  LoginSignalRepository
	policies RiskPolicyUsecase
	now      func() time.Time
}

func NewRiskUsecase(signals LoginSignalRepository, policies RiskPolicyUsecase) RiskUsecase {
	return &riskUsecase{signals: signals, policies: policies, now: time.Now}
}

func (uc *riskUsecase) Assess(ctx context.Context, in RiskInput) (domain.RiskAssessment, error) {
	if in.AccountID == "" || in.IP == "" {
		return domain.RiskAssessment{}, apierrors.InvalidArgument("account_id and ip are required")
	}

	seenIP, err := uc.signals.SeenIP(ctx, in.AccountID, in.IP)
	if err != nil {
		return domain.RiskAssessment{}, err
	}
	seenDevice, err := uc.signals.SeenDevice(ctx, in.AccountID, in.DeviceID)
	if err != nil {
		return domain.RiskAssessment{}, err
	}
	failures, err := uc.signals.RecentFailures(ctx, in.AccountID, uc.now().Add(-failureWindow))
	if err != nil {
		return domain.RiskAssessment{}, err
	}
	policy, err := uc.policies.GetPolicy(ctx, in.RealmID)
	if err != nil {
		return domain.RiskAssessment{}, err
	}

	assessment := domain.EvaluateRisk(domain.RiskContext{
		NewIP:          !seenIP,
		NewDevice:      !seenDevice,
		RecentFailures: failures,
	}, policy)

	if err := uc.signals.Record(ctx, LoginSignal{
		AccountID: in.AccountID, RealmID: in.RealmID, IP: in.IP, DeviceID: in.DeviceID, Succeeded: in.Succeeded,
	}); err != nil {
		return domain.RiskAssessment{}, err
	}
	return assessment, nil
}
