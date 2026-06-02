package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestRealmRiskPolicyController_Set(t *testing.T) {
	uc := apptest.NewRiskPolicyUsecase(t)
	uc.EXPECT().SetPolicy(mock.Anything, "r", mock.MatchedBy(func(p domain.RiskPolicy) bool {
		return p.StepUpThreshold == 40 && p.DenyThreshold == 90
	})).Return(domain.NewRealmRiskPolicy("r", domain.RiskPolicy{
		NewIPWeight: 30, NewDeviceWeight: 40, FailureWeight: 15, StepUpThreshold: 40, DenyThreshold: 90,
	}), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/realm-risk-policies returns 201",
			internaltest.NewRESTHandler(aegishttp.NewRealmRiskPolicyController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/realm-risk-policies",
				`{"data":{"type":"realmRiskPolicies","attributes":{"realmId":"r","newIpWeight":30,"newDeviceWeight":40,"failureWeight":15,"stepUpThreshold":40,"denyThreshold":90}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestRealmRiskPolicyController_GetDefaults(t *testing.T) {
	uc := apptest.NewRiskPolicyUsecase(t)
	uc.EXPECT().GetPolicy(mock.Anything, "r").Return(domain.DefaultRiskPolicy(), nil)

	rec := httptest.NewRecorder()
	internaltest.NewRESTHandler(aegishttp.NewRealmRiskPolicyController(uc)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/realm-risk-policies/r", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	// GET renders via writeJSON (raw struct, matching the realm-acr precedent);
	// assert the realm id and a weight survive the round-trip.
	assert.Contains(t, rec.Body.String(), `"r"`)
	assert.Contains(t, rec.Body.String(), "100")
}
