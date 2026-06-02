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

func TestBindingController_Create(t *testing.T) {
	uc := apptest.NewBindingUsecase(t)
	want := internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1"),
		internaltest.WithBindingSubjectType(domain.SubjectTypeAccount), internaltest.WithBindingSubjectID("acct-1"))
	uc.EXPECT().
		Create(mock.Anything, mock.MatchedBy(internaltest.MatchBinding(want))).
		Return(internaltest.NewBinding(internaltest.WithBindingID("bind-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/bindings returns 201",
			internaltest.NewRESTHandler(aegishttp.NewBindingController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/bindings",
				`{"data":{"type":"bindings","attributes":{"resourceId":"res-1","roleId":"role-1","subjectType":"ACCOUNT","subjectId":"acct-1"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestBindingController_List(t *testing.T) {
	uc := apptest.NewBindingUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.Binding{
		internaltest.NewBinding(internaltest.WithBindingID("bind-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/bindings returns 200",
			internaltest.NewRESTHandler(aegishttp.NewBindingController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/bindings", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestBindingController_Get(t *testing.T) {
	uc := apptest.NewBindingUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewBinding(internaltest.WithBindingID("bind-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/bindings/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewBindingController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/bindings/bind-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestBindingController_Delete(t *testing.T) {
	uc := apptest.NewBindingUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/bindings/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewBindingController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/bindings/bind-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
