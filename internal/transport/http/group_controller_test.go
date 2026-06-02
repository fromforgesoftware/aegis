package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestGroupController_Create(t *testing.T) {
	uc := apptest.NewGroupUsecase(t)
	want := internaltest.NewGroup(internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("editors"))
	uc.EXPECT().
		Create(mock.Anything, mock.MatchedBy(internaltest.MatchGroup(want)), []string{"acct-1", "acct-2"}).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"),
			internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("editors")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/groups returns 201",
			internaltest.NewRESTHandler(aegishttp.NewGroupController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/groups",
				`{"data":{"type":"groups","attributes":{"realmId":"r","name":"editors","members":["acct-1","acct-2"]}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestGroupController_List(t *testing.T) {
	uc := apptest.NewGroupUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.Group{
		internaltest.NewGroup(internaltest.WithGroupID("set-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/groups returns 200",
			internaltest.NewRESTHandler(aegishttp.NewGroupController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/groups", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestGroupController_Get(t *testing.T) {
	uc := apptest.NewGroupUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/groups/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewGroupController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/groups/set-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestGroupController_Delete(t *testing.T) {
	uc := apptest.NewGroupUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/groups/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewGroupController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/groups/set-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}

func TestGroupController_ListMembers(t *testing.T) {
	uc := apptest.NewGroupUsecase(t)
	uc.EXPECT().ListMembers(mock.Anything, "set-1").Return([]string{"acct-1", "acct-2"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/groups/{id}/members returns the account ids",
			internaltest.NewRESTHandler(aegishttp.NewGroupController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/groups/set-1/members", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestGroupController_SetMembers(t *testing.T) {
	uc := apptest.NewGroupUsecase(t)
	uc.EXPECT().SetMembers(mock.Anything, "set-1", []string{"acct-1"}).Return(nil)

	req := restest.NewReq(t, context.Background(), http.MethodPost, "/api/groups/set-1/members",
		strings.NewReader(`{"data":[{"type":"accounts","id":"acct-1"}]}`))
	req.Header.Set("Content-Type", "application/json")
	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/groups/{id}/members overwrites the set",
			internaltest.NewRESTHandler(aegishttp.NewGroupController(uc)),
			req,
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
