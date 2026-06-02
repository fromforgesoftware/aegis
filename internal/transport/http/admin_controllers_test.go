package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/transport/rest/restest"
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

func TestRealmController_Create(t *testing.T) {
	uc := apptest.NewRealmUsecase(t)
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.Realm) bool {
		return r.Name() == "trading-bot"
	})).Return(domain.NewRealm("trading-bot", domain.WithRealmID("realm-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/realms returns 201",
			internaltest.NewRESTHandler(aegishttp.NewRealmController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/realms",
				`{"data":{"type":"realms","attributes":{"name":"trading-bot","displayName":"Trading Bot"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestRealmController_List(t *testing.T) {
	uc := apptest.NewRealmUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.Realm{
		domain.NewRealm("trading-bot", domain.WithRealmID("realm-1")),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/realms returns 200",
			internaltest.NewRESTHandler(aegishttp.NewRealmController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/realms", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestAuditEventController_List(t *testing.T) {
	uc := apptest.NewAuditQueryUsecase(t)
	uc.EXPECT().List(mock.Anything, mock.Anything).Return(resource.NewListResponse([]domain.AuditEvent{
		domain.NewAuditEvent("binding.grant", "binding", "bind-1"),
	}, 1), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"GET /api/audit-events returns 200",
			internaltest.NewRESTHandler(aegishttp.NewAuditEventController(uc)),
			restest.NewReq(t, context.Background(), http.MethodGet, "/api/audit-events", http.NoBody),
			restest.AssertResponseStatus(http.StatusOK),
		),
	).Exec(t)
}

func TestInvitationController_Create(t *testing.T) {
	uc := apptest.NewInvitationUsecase(t)
	uc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(i domain.Invitation) bool {
		return i.Email() == "new@x.com" && i.RealmID() == "r"
	})).Return(domain.NewInvitation("r", "new@x.com", time.Now().Add(time.Hour),
		domain.WithInvitationID("inv-1")), nil)

	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/invitations returns 201",
			internaltest.NewRESTHandler(aegishttp.NewInvitationController(uc)),
			jsonapiReq(t, http.MethodPost, "/api/invitations",
				`{"data":{"type":"invitations","attributes":{"realmId":"r","email":"new@x.com","roleId":"role-1","resourceId":"res-1","expiresAt":"2030-01-01T00:00:00Z"}}}`),
			restest.AssertResponseStatus(http.StatusCreated),
		),
	).Exec(t)
}

func TestInvitationController_Accept(t *testing.T) {
	uc := apptest.NewInvitationUsecase(t)
	uc.EXPECT().Accept(mock.Anything, "raw-token", "acc-1").Return(nil)

	req := restest.NewReq(t, context.Background(), http.MethodPost, "/api/invitations/accept",
		strings.NewReader(`{"token":"raw-token","accountId":"acc-1"}`))
	req.Header.Set("Content-Type", "application/json")
	restest.NewHandlerSuite(
		restest.NewHandlerTest(
			"POST /api/invitations/accept returns 204",
			internaltest.NewRESTHandler(aegishttp.NewInvitationController(uc)),
			req,
			restest.AssertResponseStatus(http.StatusNoContent),
		),
	).Exec(t)
}
