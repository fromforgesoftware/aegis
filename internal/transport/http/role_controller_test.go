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

func TestRoleController_Create(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	want := internaltest.NewRole(
		internaltest.WithRoleRealmID("r"),
		internaltest.WithRoleName("editor"),
		internaltest.WithRoleResourceType("doc"),
	)
	uc.EXPECT().
		Create(mock.Anything, mock.MatchedBy(internaltest.MatchRole(want)), []string{"doc.read", "doc.write"}).
		Return(internaltest.NewRole(
			internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("r"),
			internaltest.WithRoleName("editor"),
			internaltest.WithRoleResourceType("doc"),
		), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/roles returns 201",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/roles",
				`{"data":{"type":"roles","attributes":{"realmId":"r","name":"editor","resourceType":"doc","kind":"CUSTOM","permissions":["doc.read","doc.write"]}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestRoleController_List(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.Role{
		internaltest.NewRole(internaltest.WithRoleID("role-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/roles returns 200",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/roles", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestRoleController_Get(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/roles/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/roles/role-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestRoleController_Delete(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/roles/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/roles/role-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}

func TestRoleController_ListPermissions(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().ListPermissions(mock.Anything, "role-1").Return([]string{"doc.read", "doc.write"}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/roles/{id}/permissions returns the slugs",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/roles/role-1/permissions", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestRoleController_SetComposition(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().SetComposition(mock.Anything, "editor", []domain.RoleComponent{
		{ComponentRoleID: "viewer", Operator: domain.CompositionUnion, Ordinal: 0},
	}).Return(nil)

	req := restest.NewReq(t, context.Background(), http.MethodPost, "/api/roles/editor/composition",
		strings.NewReader(`{"data":[{"componentRoleId":"viewer","operator":"UNION","ordinal":0}]}`))
	req.Header.Set("Content-Type", "application/json")
	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/roles/{id}/composition overwrites the composition",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			req,
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}

func TestRoleController_ListComposition(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().ListComposition(mock.Anything, "editor").Return([]domain.RoleComponent{
		{ComponentRoleID: "viewer", Operator: domain.CompositionUnion, Ordinal: 0},
	}, nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/roles/{id}/composition returns the components",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/roles/editor/composition", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestRoleController_SetPermissions(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().SetPermissions(mock.Anything, "role-1", []string{"doc.read"}).Return(nil)

	req := restest.NewReq(t, context.Background(), http.MethodPost, "/api/roles/role-1/permissions",
		strings.NewReader(`{"data":[{"type":"permissions","id":"doc.read"}]}`))
	req.Header.Set("Content-Type", "application/json")
	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/roles/{id}/permissions overwrites the set",
			internaltest.NewRESTHandler(aegishttp.NewRoleController(uc)),
			req,
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
