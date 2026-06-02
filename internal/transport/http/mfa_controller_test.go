package http_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestMFAController_Enroll(t *testing.T) {
	uc := apptest.NewMFAUsecase(t)
	uc.EXPECT().EnrollTOTP(mock.Anything, "acc-1", "Aegis", "user@x.com").
		Return("SECRET", "otpauth://totp/Aegis:user", nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/mfa/enroll returns 201",
			internaltest.NewRESTHandler(aegishttp.NewMFAController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/mfa/enroll",
				`{"data":{"type":"mfaEnrollments","attributes":{"accountId":"acc-1","issuer":"Aegis","accountLabel":"user@x.com"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestMFAController_Verify(t *testing.T) {
	uc := apptest.NewMFAUsecase(t)
	uc.EXPECT().VerifyTOTP(mock.Anything, "acc-1", "123456").Return(true, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/mfa/verify returns 200",
			internaltest.NewRESTHandler(aegishttp.NewMFAController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/mfa/verify",
				`{"data":{"type":"mfaVerifications","attributes":{"accountId":"acc-1","code":"123456"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestMFAController_StepUp(t *testing.T) {
	uc := apptest.NewMFAUsecase(t)
	uc.EXPECT().StepUp(mock.Anything, "acc-1", domain.MFAFactorTOTP, "123456").
		Return("tok", "aal2", time.Now().Add(time.Minute), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/mfa/step-up returns 200",
			internaltest.NewRESTHandler(aegishttp.NewMFAController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/mfa/step-up",
				`{"data":{"type":"stepUps","attributes":{"accountId":"acc-1","factor":"TOTP","proof":"123456"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestRealmACRPolicyController_Set(t *testing.T) {
	uc := apptest.NewMFAPolicyUsecase(t)
	uc.EXPECT().SetPolicy(mock.Anything, mock.MatchedBy(func(p domain.RealmACRPolicy) bool {
		return p.RealmID() == "r" && p.MFARequired()
	})).Return(domain.NewRealmACRPolicy("r", true, "aal2"), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/realm-acr-policies returns 201",
			internaltest.NewRESTHandler(aegishttp.NewRealmACRPolicyController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/realm-acr-policies",
				`{"data":{"type":"realmAcrPolicies","attributes":{"realmId":"r","mfaRequired":true,"requiredAcr":"aal2"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}
