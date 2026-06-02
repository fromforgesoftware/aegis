package http_test

import (
	"context"
	"net/http"
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

func TestAuthzResourceController_Create(t *testing.T) {
	uc := apptest.NewAuthzResourceUsecase(t)
	want := internaltest.NewAuthzResource(
		internaltest.WithAuthzResourceRealmID("r"),
		internaltest.WithAuthzResourceResourceType("workspace"),
	)
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchAuthzResource(want))).
		Return(internaltest.NewAuthzResource(
			internaltest.WithAuthzResourceID("ws-1"),
			internaltest.WithAuthzResourceRealmID("r"),
			internaltest.WithAuthzResourceResourceType("workspace"),
		), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/resources returns 201",
			internaltest.NewRESTHandler(aegishttp.NewAuthzResourceController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/resources",
				`{"data":{"type":"resources","attributes":{"realmId":"r","type":"workspace","visibility":"PRIVATE"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestAuthzResourceController_Get(t *testing.T) {
	uc := apptest.NewAuthzResourceUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("ws-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/resources/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuthzResourceController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/resources/ws-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuthzResourceController_List(t *testing.T) {
	uc := apptest.NewAuthzResourceUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.AuthzResource{
		internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("ws-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/resources returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuthzResourceController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/resources", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuthzResourceController_Delete(t *testing.T) {
	uc := apptest.NewAuthzResourceUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/resources/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewAuthzResourceController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/resources/ws-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
