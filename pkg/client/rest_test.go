package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
	"github.com/fromforgesoftware/aegis/pkg/client"
)

// newHTTPClient serves the real controller behind httptest so the SDK is
// exercised against the actual wire shapes, and returns the shared transport.
func newHTTPClient(t *testing.T, controller kitrest.Controller) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(kitrest.BuildHandler(controller))
	t.Cleanup(server.Close)
	return server
}

func TestBindingHTTPClient_CreateRoundTrips(t *testing.T) {
	uc := apptest.NewBindingUsecase(t)
	uc.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(b domain.Binding) bool {
			return b.ResourceID() == "res-1" && b.RoleID() == "role-1" &&
				b.SubjectType() == domain.SubjectTypeAccount && b.SubjectID() == "acct-1"
		})).
		Return(internaltest.NewBinding(internaltest.WithBindingID("bind-1")), nil)

	server := newHTTPClient(t, aegishttp.NewBindingController(uc))
	rest, err := client.NewRESTClient(client.Config{HTTPURL: server.URL})
	require.NoError(t, err)

	created, err := client.NewBindingHTTPClient(rest).Create(context.Background(), client.Binding{
		ResourceID: "res-1", RoleID: "role-1",
		SubjectType: client.SubjectAccount, SubjectID: "acct-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "bind-1", created.ID)
}

func TestBindingHTTPClient_MapsAPIError(t *testing.T) {
	uc := apptest.NewBindingUsecase(t)
	uc.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("binding", "missing"))

	server := newHTTPClient(t, aegishttp.NewBindingController(uc))
	rest, err := client.NewRESTClient(client.Config{HTTPURL: server.URL})
	require.NoError(t, err)

	_, err = client.NewBindingHTTPClient(rest).Get(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound))
}

func TestRoleHTTPClient_PermissionsRoundTrip(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().SetPermissions(mock.Anything, "role-1", []string{"doc.read", "doc.write"}).Return(nil)
	uc.EXPECT().ListPermissions(mock.Anything, "role-1").Return([]string{"doc.read", "doc.write"}, nil)

	server := newHTTPClient(t, aegishttp.NewRoleController(uc))
	rest, err := client.NewRESTClient(client.Config{HTTPURL: server.URL})
	require.NoError(t, err)
	roles := client.NewRoleHTTPClient(rest)

	require.NoError(t, roles.SetPermissions(context.Background(), "role-1", []string{"doc.read", "doc.write"}))
	perms, err := roles.Permissions(context.Background(), "role-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"doc.read", "doc.write"}, perms)
}

func TestRoleHTTPClient_CreateSeedsPermissions(t *testing.T) {
	uc := apptest.NewRoleUsecase(t)
	uc.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(r domain.Role) bool {
			return r.RealmID() == "realm-1" && r.Name() == "strategy-owner"
		}), []string{"strategy.read"}).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"), internaltest.WithRoleName("strategy-owner")), nil)

	server := newHTTPClient(t, aegishttp.NewRoleController(uc))
	rest, err := client.NewRESTClient(client.Config{HTTPURL: server.URL})
	require.NoError(t, err)

	created, err := client.NewRoleHTTPClient(rest).Create(context.Background(), client.Role{
		RealmID: "realm-1", Name: "strategy-owner", ResourceType: "strategy",
	}, []string{"strategy.read"})
	require.NoError(t, err)
	assert.Equal(t, "role-1", created.ID)
	assert.Equal(t, "strategy-owner", created.Name)
}

func TestRESTClient_SendsGatewayBearer(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	rest, err := client.NewRESTClient(client.Config{HTTPURL: server.URL, GatewaySecret: "shh", ServiceName: "test"})
	require.NoError(t, err)

	require.NoError(t, client.NewAuthorizationAdminHTTPClient(rest).Refresh(context.Background()))
	assert.Contains(t, gotAuth, "Bearer ")
}
