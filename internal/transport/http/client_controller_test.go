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

func TestClientController_Create(t *testing.T) {
	uc := apptest.NewClientUsecase(t)
	want := domain.NewClient("r", "web", domain.ClientTypeConfidential, "Web App")
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchClient(want))).
		Return(domain.NewClient("r", "web", domain.ClientTypeConfidential, "Web App",
			domain.WithClientID("c-1"), domain.WithClientSecret("raw-secret")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/clients returns 201",
			internaltest.NewRESTHandler(aegishttp.NewClientController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/clients",
				`{"data":{"type":"clients","attributes":{"realmId":"r","clientId":"web","type":"CONFIDENTIAL","name":"Web App"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestClientController_Get(t *testing.T) {
	uc := apptest.NewClientUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewClient("r", "web", domain.ClientTypePublic, "Web App", domain.WithClientID("c-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/clients/{id} returns 200",
			internaltest.NewRESTHandler(aegishttp.NewClientController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/clients/c-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestClientController_List(t *testing.T) {
	uc := apptest.NewClientUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.Client{
		domain.NewClient("r", "web", domain.ClientTypePublic, "Web App", domain.WithClientID("c-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/clients returns 200",
			internaltest.NewRESTHandler(aegishttp.NewClientController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/clients", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestClientController_Delete(t *testing.T) {
	uc := apptest.NewClientUsecase(t)
	uc.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"DELETE /api/clients/{id} returns 204",
			internaltest.NewRESTHandler(aegishttp.NewClientController(uc)),
			restest.NewReq(t, context.Background(), http.MethodDelete, "/api/clients/c-1", http.NoBody),
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
