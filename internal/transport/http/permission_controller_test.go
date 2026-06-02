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

func TestPermissionController_Create(t *testing.T) {
	uc := apptest.NewPermissionUsecase(t)
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p domain.Permission) bool {
		return p.ID() == "doc.read" && p.ResourceType() == "doc" && p.Verb() == "read"
	})).Return(domain.NewPermission("doc.read", "doc", "read"), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/permissions returns 201",
			internaltest.NewRESTHandler(aegishttp.NewPermissionController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/permissions",
				`{"data":{"type":"permissions","id":"doc.read","attributes":{"resourceType":"doc","verb":"read"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestPermissionController_List(t *testing.T) {
	uc := apptest.NewPermissionUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.Permission{
		domain.NewPermission("doc.read", "doc", "read"),
		domain.NewPermission("doc.write", "doc", "write"),
	}, 2), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/permissions returns 200",
			internaltest.NewRESTHandler(aegishttp.NewPermissionController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/permissions", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestPermissionController_Get(t *testing.T) {
	uc := apptest.NewPermissionUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.NewPermission("doc.read", "doc", "read"), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/permissions/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewPermissionController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/permissions/doc.read", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestPermissionController_SetImplications(t *testing.T) {
	uc := apptest.NewPermissionUsecase(t)
	uc.EXPECT().SetImplications(mock.Anything, "doc.write", []string{"doc.read"}).Return(nil)

	req := restest.NewReq(t, context.Background(), http.MethodPost, "/api/permissions/doc.write/implications",
		strings.NewReader(`{"data":[{"type":"permissions","id":"doc.read"}]}`))
	req.Header.Set("Content-Type", "application/json")
	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/permissions/{id}/implications overwrites the set",
			internaltest.NewRESTHandler(aegishttp.NewPermissionController(uc)),
			req,
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}

func TestPermissionController_ListImplications(t *testing.T) {
	uc := apptest.NewPermissionUsecase(t)
	uc.EXPECT().ListImplications(mock.Anything, "doc.write").Return([]string{"doc.read"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/permissions/{id}/implications returns the slugs",
			internaltest.NewRESTHandler(aegishttp.NewPermissionController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/permissions/doc.write/implications", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestPermissionController_Delete(t *testing.T) {
	uc := apptest.NewPermissionUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeHard, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/permissions/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewPermissionController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/permissions/doc.read", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
