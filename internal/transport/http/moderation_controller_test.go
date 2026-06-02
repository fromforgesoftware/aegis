package http_test

import (
	"net/http"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestModerationController_Ban(t *testing.T) {
	uc := apptest.NewAccountModerationUsecase(t)
	merge := apptest.NewAccountMergeUsecase(t)
	uc.EXPECT().Ban(mock.Anything, "acc-1", mock.Anything, "spam").Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/accounts/ban returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAccountModerationController(uc, merge)),
			jsonapiReq(t, http.MethodPost, "/api/accounts/ban",
				`{"data":{"type":"accountBans","attributes":{"accountId":"acc-1","reason":"spam"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestModerationController_Unban(t *testing.T) {
	uc := apptest.NewAccountModerationUsecase(t)
	merge := apptest.NewAccountMergeUsecase(t)
	uc.EXPECT().Unban(mock.Anything, "acc-1").Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/accounts/unban returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAccountModerationController(uc, merge)),
			jsonapiReq(t, http.MethodPost, "/api/accounts/unban",
				`{"data":{"type":"accountBans","attributes":{"accountId":"acc-1"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestModerationController_Merge(t *testing.T) {
	uc := apptest.NewAccountModerationUsecase(t)
	merge := apptest.NewAccountMergeUsecase(t)
	merge.EXPECT().Merge(mock.Anything, "src", "dst").
		Return(app.MergeSummary{ExternalIDs: 1, Memberships: 2, Bindings: 3}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/accounts/merge returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAccountModerationController(uc, merge)),
			jsonapiReq(t, http.MethodPost, "/api/accounts/merge",
				`{"data":{"type":"accountMerges","attributes":{"sourceId":"src","targetId":"dst"}}}`),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}
