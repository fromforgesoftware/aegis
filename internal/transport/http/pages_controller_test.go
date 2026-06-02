package http_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func formReq(t *testing.T, target string, form url.Values) *http.Request {
	req := restest.NewReq(t, context.Background(), http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestPagesController_StartLogin(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	want := internaltest.NewFlow(internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeLogin))
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchFlow(want))).
		Return(internaltest.NewFlow(internaltest.WithFlowID("flow-1"), internaltest.WithFlowType(domain.FlowTypeLogin)), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /auth/login renders the form (200)",
			internaltest.NewRESTHandler(aegishttp.NewPagesController(uc, apptest.NewOAuthUsecase(t), resolvingRealms(t, "r"), permissiveRL())),
			restest.NewReq(t, context.Background(), http.MethodGet, "/auth/login?realm=r", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestPagesController_SubmitSuccess(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	uc.EXPECT().Submit(mock.Anything, app.SubmitFlowInput{
		FlowID:  "flow-1",
		Payload: map[string]string{"email": "a@b.com", "password": "password123"},
	}).Return(internaltest.NewFlow(internaltest.WithFlowState(domain.FlowStateCompleted)), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /auth/login completes the flow (200)",
			internaltest.NewRESTHandler(aegishttp.NewPagesController(uc, apptest.NewOAuthUsecase(t), resolvingRealms(t, "r"), permissiveRL())),
			formReq(t, "/auth/login", url.Values{"flow": {"flow-1"}, "email": {"a@b.com"}, "password": {"password123"}}),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestPagesController_LoginStartsSessionAndRedirects(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	uc.EXPECT().Submit(mock.Anything, app.SubmitFlowInput{
		FlowID:  "flow-1",
		Payload: map[string]string{"email": "a@b.com", "password": "password123"},
	}).Return(internaltest.NewFlow(
		internaltest.WithFlowRealmID("r"),
		internaltest.WithFlowType(domain.FlowTypeLogin),
		internaltest.WithFlowState(domain.FlowStateCompleted),
		internaltest.WithFlowResultAccountID("acc-1"),
	), nil)

	oauth := apptest.NewOAuthUsecase(t)
	oauth.EXPECT().StartSession(mock.Anything, "r", "acc-1").Return("sess-1", nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /auth/login establishes a session and redirects to return_to (302)",
			internaltest.NewRESTHandler(aegishttp.NewPagesController(uc, oauth, resolvingRealms(t, "r"), permissiveRL())),
			formReq(t, "/auth/login", url.Values{
				"flow":      {"flow-1"},
				"email":     {"a@b.com"},
				"password":  {"password123"},
				"return_to": {"/realms/r/authorize?client_id=web"},
			}),
			restest.AssertResponseStatus(http.StatusFound),
		),
	).Exec(t)
}

func TestPagesController_SubmitError(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	uc.EXPECT().Submit(mock.Anything, app.SubmitFlowInput{
		FlowID:  "flow-1",
		Payload: map[string]string{"email": "a@b.com", "password": "wrong"},
	}).Return(nil, apierrors.Unauthenticated("invalid credentials"))

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /auth/login re-renders with the auth error status (401)",
			internaltest.NewRESTHandler(aegishttp.NewPagesController(uc, apptest.NewOAuthUsecase(t), resolvingRealms(t, "r"), permissiveRL())),
			formReq(t, "/auth/login", url.Values{"flow": {"flow-1"}, "email": {"a@b.com"}, "password": {"wrong"}}),
			restest.AssertResponseStatus(http.StatusUnauthorized),
		),
	).Exec(t)
}

func TestPagesController_UnknownType(t *testing.T) {
	uc := apptest.NewFlowUsecase(t)
	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /auth/{unknown} is 404",
			internaltest.NewRESTHandler(aegishttp.NewPagesController(uc, apptest.NewOAuthUsecase(t), resolvingRealms(t, "r"), permissiveRL())),
			restest.NewReq(t, context.Background(), http.MethodGet, "/auth/bogus", http.NoBody),
			restest.AssertResponseStatus(http.StatusNotFound),
		),
	).Exec(t)
}
