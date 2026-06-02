package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// defaultPolicyUsecase returns a RiskPolicyUsecase whose realm lookups always
// fall back to the service default — the common case for the risk-eval tests.
func defaultPolicyUsecase(t *testing.T) app.RiskPolicyUsecase {
	repo := apptest.NewRealmRiskPolicyRepository(t)
	repo.EXPECT().GetByRealm(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("realm risk policy", "")).Maybe()
	return app.NewRiskPolicyUsecase(repo)
}

func TestRiskAssess_NewIPAndDeviceStepsUpAndRecords(t *testing.T) {
	signals := apptest.NewLoginSignalRepository(t)
	uc := app.NewRiskUsecase(signals, defaultPolicyUsecase(t))

	signals.EXPECT().SeenIP(mock.Anything, "acc-1", "1.2.3.4").Return(false, nil)
	signals.EXPECT().SeenDevice(mock.Anything, "acc-1", "dev-x").Return(false, nil)
	signals.EXPECT().RecentFailures(mock.Anything, "acc-1", mock.Anything).Return(0, nil)
	// The current attempt is recorded after assessment, so it never counts itself.
	signals.EXPECT().Record(mock.Anything, mock.MatchedBy(func(s app.LoginSignal) bool {
		return s.AccountID == "acc-1" && s.IP == "1.2.3.4"
	})).Return(nil)

	got, err := uc.Assess(context.Background(), app.RiskInput{
		AccountID: "acc-1", IP: "1.2.3.4", DeviceID: "dev-x", Succeeded: true,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.RiskStepUp, got.Decision)
}

func TestRiskAssess_KnownIPAndDeviceAllows(t *testing.T) {
	signals := apptest.NewLoginSignalRepository(t)
	uc := app.NewRiskUsecase(signals, defaultPolicyUsecase(t))

	signals.EXPECT().SeenIP(mock.Anything, "acc-1", "1.2.3.4").Return(true, nil)
	signals.EXPECT().SeenDevice(mock.Anything, "acc-1", "dev-x").Return(true, nil)
	signals.EXPECT().RecentFailures(mock.Anything, "acc-1", mock.Anything).Return(0, nil)
	signals.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	got, err := uc.Assess(context.Background(), app.RiskInput{AccountID: "acc-1", IP: "1.2.3.4", DeviceID: "dev-x"})
	require.NoError(t, err)
	assert.Equal(t, domain.RiskAllow, got.Decision)
}

func TestRiskAssess_RejectsMissingFields(t *testing.T) {
	uc := app.NewRiskUsecase(apptest.NewLoginSignalRepository(t), defaultPolicyUsecase(t))
	_, err := uc.Assess(context.Background(), app.RiskInput{AccountID: "", IP: ""})
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRiskAssess_PerRealmPolicyOverridesThresholds(t *testing.T) {
	signals := apptest.NewLoginSignalRepository(t)
	// A strict realm: a new IP alone (weight 60) crosses the step-up threshold (50).
	repo := apptest.NewRealmRiskPolicyRepository(t)
	repo.EXPECT().GetByRealm(mock.Anything, "strict").Return(
		domain.NewRealmRiskPolicy("strict", domain.RiskPolicy{
			NewIPWeight: 60, NewDeviceWeight: 60, FailureWeight: 20, StepUpThreshold: 50, DenyThreshold: 200,
		}), nil)
	uc := app.NewRiskUsecase(signals, app.NewRiskPolicyUsecase(repo))

	signals.EXPECT().SeenIP(mock.Anything, "acc-1", "1.2.3.4").Return(false, nil)
	signals.EXPECT().SeenDevice(mock.Anything, "acc-1", "dev-x").Return(true, nil)
	signals.EXPECT().RecentFailures(mock.Anything, "acc-1", mock.Anything).Return(0, nil)
	signals.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	got, err := uc.Assess(context.Background(), app.RiskInput{RealmID: "strict", AccountID: "acc-1", IP: "1.2.3.4", DeviceID: "dev-x"})
	require.NoError(t, err)
	assert.Equal(t, domain.RiskStepUp, got.Decision, "new IP alone steps up under the strict realm policy")
}

func TestRiskPolicyUsecase_SetValidatesAndGetDefaults(t *testing.T) {
	repo := apptest.NewRealmRiskPolicyRepository(t)
	uc := app.NewRiskPolicyUsecase(repo)

	_, err := uc.SetPolicy(context.Background(), "", domain.DefaultRiskPolicy())
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))

	// Unset realm → service default.
	repo.EXPECT().GetByRealm(mock.Anything, "r").Return(nil, apierrors.NotFound("realm risk policy", "r"))
	got, err := uc.GetPolicy(context.Background(), "r")
	require.NoError(t, err)
	assert.Equal(t, domain.DefaultRiskPolicy(), got)
}
