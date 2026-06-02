package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
	aegisgrpc "github.com/fromforgesoftware/aegis/internal/transport/grpc"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

func TestIdentityBrokerController_ResolveAccount(t *testing.T) {
	broker := apptest.NewIdentityBrokerUsecase(t)
	broker.EXPECT().ResolveAccount(mock.Anything, app.ResolveAccountInput{
		RealmID: "r", IDPName: "google-prod", RawToken: "tok",
	}).Return(app.ResolveAccountResult{
		Account: internaltest.NewAccount(
			internaltest.WithAccountID("acc-1"),
			internaltest.WithAccountEmail("a@b.com"),
		),
		Created: true,
	}, nil)

	ctrl := aegisgrpc.NewIdentityBrokerController(broker).(aegisv1.IdentityBrokerServiceServer)
	resp, err := ctrl.ResolveAccount(context.Background(), &aegisv1.ResolveAccountRequest{
		RealmId: "r", IdpName: "google-prod", Token: "tok",
	})
	require.NoError(t, err)
	assert.Equal(t, "acc-1", resp.GetAccountId())
	assert.Equal(t, "a@b.com", resp.GetEmail())
	assert.True(t, resp.GetCreated())
	assert.False(t, resp.GetLinkRequired())
}

func TestIdentityBrokerController_LinkRequired(t *testing.T) {
	broker := apptest.NewIdentityBrokerUsecase(t)
	broker.EXPECT().ResolveAccount(mock.Anything, app.ResolveAccountInput{
		RealmID: "r", IDPName: "github-prod", RawToken: "ghp_TOKEN",
	}).Return(app.ResolveAccountResult{
		Account:      internaltest.NewAccount(internaltest.WithAccountID("acc-existing")),
		LinkRequired: true,
	}, nil)

	ctrl := aegisgrpc.NewIdentityBrokerController(broker).(aegisv1.IdentityBrokerServiceServer)
	resp, err := ctrl.ResolveAccount(context.Background(), &aegisv1.ResolveAccountRequest{
		RealmId: "r", IdpName: "github-prod", Token: "ghp_TOKEN",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetLinkRequired())
	assert.Equal(t, "acc-existing", resp.GetAccountId())
}
