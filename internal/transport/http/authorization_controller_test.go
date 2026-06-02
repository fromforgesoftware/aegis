package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestAuthorizationController_Refresh(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().Refresh(mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/authorizations/refresh returns 204",
			internaltest.NewRESTHandler(aegishttp.NewAuthorizationController(uc, apptest.NewGrantSweeper(t))),
			restest.NewReq(t, context.Background(), http.MethodPost, "/api/authorizations/refresh", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}

func TestAuthorizationController_Sweep(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	sweeper := apptest.NewGrantSweeper(t)
	sweeper.EXPECT().Sweep(mock.Anything).Return(int64(3), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/authorizations/sweep returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuthorizationController(uc, sweeper)),
			restest.NewReq(t, context.Background(), http.MethodPost, "/api/authorizations/sweep", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuthorizationController_Check(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().Check(mock.Anything, "acct-1", "res-1", "doc.read", int64(0)).Return(true, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/authorizations/check returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuthorizationController(uc, apptest.NewGrantSweeper(t))),
			jsonapiReq(t, http.MethodPost, "/api/authorizations/check",
				`{"data":{"type":"authorizationChecks","attributes":{"accountId":"acct-1","resourceId":"res-1","permissionId":"doc.read"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuthorizationController_BatchCheck(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().BatchCheck(mock.Anything, "acct-1", []domain.PermissionCheck{
		{ResourceID: "res-1", PermissionID: "doc.read"},
	}, int64(0)).Return([]domain.PermissionDecision{
		{ResourceID: "res-1", PermissionID: "doc.read", Allowed: true},
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/authorizations/batch-check returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuthorizationController(uc, apptest.NewGrantSweeper(t))),
			jsonapiReq(t, http.MethodPost, "/api/authorizations/batch-check",
				`{"data":{"type":"authorizationBatchChecks","attributes":{"accountId":"acct-1","checks":[{"resourceId":"res-1","permissionId":"doc.read"}]}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuthorizationController_Version(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().Version(mock.Anything).Return(int64(9), int64(5), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/authorizations/version returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuthorizationController(uc, apptest.NewGrantSweeper(t))),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/authorizations/version", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuthorizationController_ListAccessible(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().ListAccessible(mock.Anything, "acct-1", "doc.read", int64(0)).
		Return([]string{"res-1", "res-2"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/authorizations/accessible returns the resource ids",
			internaltest.NewRESTHandler(aegishttp.NewAuthorizationController(uc, apptest.NewGrantSweeper(t))),
			restest.NewReq(t, context.Background(), http.MethodGet,
				"/api/authorizations/accessible?accountId=acct-1&permissionId=doc.read", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
