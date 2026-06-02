package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	aegisgrpc "github.com/fromforgesoftware/aegis/internal/transport/grpc"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

func TestAuthorizerController_Check(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().Check(mock.Anything, "acct-1", "res-1", "doc.read", int64(0)).Return(true, nil)

	ctrl := aegisgrpc.NewAuthorizerController(uc).(aegisv1.AuthorizerServiceServer)
	resp, err := ctrl.Check(context.Background(), &aegisv1.CheckRequest{
		AccountId: "acct-1", ResourceId: "res-1", PermissionId: "doc.read",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetAllowed())
}

func TestAuthorizerController_CheckPassesMinVersion(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().Check(mock.Anything, "acct-1", "res-1", "doc.read", int64(42)).Return(true, nil)

	ctrl := aegisgrpc.NewAuthorizerController(uc).(aegisv1.AuthorizerServiceServer)
	_, err := ctrl.Check(context.Background(), &aegisv1.CheckRequest{
		AccountId: "acct-1", ResourceId: "res-1", PermissionId: "doc.read", MinVersion: 42,
	})
	require.NoError(t, err)
}

func TestAuthorizerController_BatchCheck(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().BatchCheck(mock.Anything, "acct-1", []domain.PermissionCheck{
		{ResourceID: "res-1", PermissionID: "doc.read"},
		{ResourceID: "res-1", PermissionID: "doc.write"},
	}, int64(0)).Return([]domain.PermissionDecision{
		{ResourceID: "res-1", PermissionID: "doc.read", Allowed: true},
		{ResourceID: "res-1", PermissionID: "doc.write", Allowed: false},
	}, nil)

	ctrl := aegisgrpc.NewAuthorizerController(uc).(aegisv1.AuthorizerServiceServer)
	resp, err := ctrl.BatchCheck(context.Background(), &aegisv1.BatchCheckRequest{
		AccountId: "acct-1",
		Checks: []*aegisv1.PermissionCheck{
			{ResourceId: "res-1", PermissionId: "doc.read"},
			{ResourceId: "res-1", PermissionId: "doc.write"},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetDecisions(), 2)
	assert.True(t, resp.GetDecisions()[0].GetAllowed())
	assert.False(t, resp.GetDecisions()[1].GetAllowed())
}

func TestAuthorizerController_ListAccessible(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().ListAccessible(mock.Anything, "acct-1", "doc.read", int64(0)).
		Return([]string{"res-1", "res-2"}, nil)

	ctrl := aegisgrpc.NewAuthorizerController(uc).(aegisv1.AuthorizerServiceServer)
	resp, err := ctrl.ListAccessible(context.Background(), &aegisv1.ListAccessibleRequest{
		AccountId: "acct-1", PermissionId: "doc.read",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"res-1", "res-2"}, resp.GetResourceIds())
}

func TestAuthorizerController_Version(t *testing.T) {
	uc := apptest.NewAuthorizationUsecase(t)
	uc.EXPECT().Version(mock.Anything).Return(int64(9), int64(5), nil)

	ctrl := aegisgrpc.NewAuthorizerController(uc).(aegisv1.AuthorizerServiceServer)
	resp, err := ctrl.Version(context.Background(), &aegisv1.VersionRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(9), resp.GetWriteVersion())
	assert.Equal(t, int64(5), resp.GetProjectionVersion())
}
