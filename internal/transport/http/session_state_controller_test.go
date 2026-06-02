package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestSessionStateController_Track(t *testing.T) {
	uc := apptest.NewSessionStateUsecase(t)
	want := internaltest.NewSessionState(
		internaltest.WithSessionStateSessionID("sess-1"),
		internaltest.WithSessionStateAccountID("acc-1"),
		internaltest.WithSessionStateCurrentShard("silvermoon"),
	)
	uc.EXPECT().Track(mock.Anything, mock.MatchedBy(internaltest.MatchSessionState(want))).Return(want, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/session-states returns 200",
			internaltest.NewRESTHandler(aegishttp.NewSessionStateController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/session-states",
				`{"data":{"type":"sessionStates","id":"sess-1","attributes":{"accountId":"acc-1","currentShard":"silvermoon"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestSessionStateController_Get(t *testing.T) {
	uc := apptest.NewSessionStateUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewSessionState(internaltest.WithSessionStateSessionID("sess-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/session-states/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewSessionStateController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/session-states/sess-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestSessionStateController_List(t *testing.T) {
	uc := apptest.NewSessionStateUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.SessionState{
		internaltest.NewSessionState(internaltest.WithSessionStateSessionID("sess-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/session-states returns 200",
			internaltest.NewRESTHandler(aegishttp.NewSessionStateController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/session-states", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestSessionStateController_Touch(t *testing.T) {
	uc := apptest.NewSessionStateUsecase(t)
	uc.EXPECT().Touch(mock.Anything, "sess-1").Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/session-states/{id}/touch returns 204",
			internaltest.NewRESTHandler(aegishttp.NewSessionStateController(uc)),
			restest.NewReq(t, context.Background(), http.MethodPost, "/api/session-states/sess-1/touch", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}

func TestQuotaPolicyController_Set(t *testing.T) {
	uc := apptest.NewQuotaUsecase(t)
	want := internaltest.NewQuotaPolicy(
		internaltest.WithQuotaPolicyRealmID("r"),
		internaltest.WithQuotaPolicyResourceType("character"),
		internaltest.WithQuotaPolicyMaxCount(3),
	)
	uc.EXPECT().SetPolicy(mock.Anything, mock.MatchedBy(internaltest.MatchQuotaPolicy(want))).Return(want, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/quota-policies returns 201",
			internaltest.NewRESTHandler(aegishttp.NewQuotaPolicyController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/quota-policies",
				`{"data":{"type":"quotaPolicies","attributes":{"realmId":"r","resourceType":"character","maxCount":3}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestQuotaPolicyController_Check(t *testing.T) {
	uc := apptest.NewQuotaUsecase(t)
	uc.EXPECT().Allow(mock.Anything, "r", "character", 2).Return(true, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/quota-policies/check returns 200",
			internaltest.NewRESTHandler(aegishttp.NewQuotaPolicyController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/quota-policies/check",
				`{"data":{"type":"quotaChecks","attributes":{"realmId":"r","resourceType":"character","current":2}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
