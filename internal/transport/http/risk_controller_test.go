package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestRiskController_Assess(t *testing.T) {
	uc := apptest.NewRiskUsecase(t)
	uc.EXPECT().Assess(mock.Anything, mock.MatchedBy(func(in app.RiskInput) bool {
		return in.AccountID == "acc-1" && in.IP == "1.2.3.4"
	})).Return(domain.RiskAssessment{
		Score: 70, Level: domain.RiskLevelMedium, Decision: domain.RiskStepUp, Reasons: []string{"new_ip", "new_device"},
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/risk/assess",
		strings.NewReader(`{"data":{"type":"riskAssessments","attributes":{"accountId":"acc-1","ip":"1.2.3.4","deviceId":"dev-x"}}}`))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	internaltest.NewRESTHandler(aegishttp.NewRiskController(uc)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out struct {
		Data struct {
			Attributes struct {
				Decision string `json:"decision"`
				Score    int    `json:"score"`
			} `json:"attributes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "STEP_UP", out.Data.Attributes.Decision)
	assert.Equal(t, 70, out.Data.Attributes.Score)
}
