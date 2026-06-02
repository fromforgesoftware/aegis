package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func jsonapiReq(t *testing.T, method, target, body string) *http.Request {
	req := restest.NewReq(t, context.Background(), method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	return req
}

// permissiveRL is a rate limiter wide enough never to trip in tests; the
// per-IP limiter itself is unit-tested in the kit.
func permissiveRL() *kitrest.RateLimitMiddleware {
	return kitrest.NewRateLimitMiddleware(1000, 1000)
}

func TestFlowController_Start(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	want := internaltest.NewFlow(internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeLogin))
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchFlow(want))).
		Return(internaltest.NewFlow(internaltest.WithFlowID("flow-1"), internaltest.WithFlowType(domain.FlowTypeLogin)), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/auth/flows returns 201 jsonapi document",
			internaltest.NewRESTHandler(aegishttp.NewFlowController(uc, permissiveRL())),
			jsonapiReq(t, http.MethodPost, "/api/auth/flows",
				`{"data":{"type":"authFlows","attributes":{"realmId":"r","flowType":"LOGIN"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestFlowController_Get(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewFlow(internaltest.WithFlowID("flow-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/auth/flows/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewFlowController(uc, permissiveRL())),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/auth/flows/flow-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestFlowController_Submit(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	uc.EXPECT().Submit(mock.Anything, app.SubmitFlowInput{
		FlowID:  "flow-1",
		Payload: map[string]string{"email": "a@b.com", "password": "password123"},
	}).Return(internaltest.NewFlow(internaltest.WithFlowID("flow-1"), internaltest.WithFlowState(domain.FlowStateCompleted)), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"PATCH /api/auth/flows/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewFlowController(uc, permissiveRL())),
			jsonapiReq(t, http.MethodPatch, "/api/auth/flows/flow-1",
				`{"data":{"type":"authFlows","id":"flow-1","attributes":{"email":"a@b.com","password":"password123"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestFlowController_SubmitError(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	uc.EXPECT().Submit(mock.Anything, mock.Anything).
		Return(nil, apierrors.InvalidArgument("flow has expired"))

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"submit on an invalid flow maps to 400",
			internaltest.NewRESTHandler(aegishttp.NewFlowController(uc, permissiveRL())),
			jsonapiReq(t, http.MethodPatch, "/api/auth/flows/flow-1",
				`{"data":{"type":"authFlows","id":"flow-1","attributes":{"email":"a@b.com","password":"password123"}}}`),
			restest.AssertResponseStatus(http.StatusBadRequest),
		),
	).Exec(t)
}
