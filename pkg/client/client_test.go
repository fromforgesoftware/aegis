package client_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
	"github.com/fromforgesoftware/aegis/pkg/client"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport/grpc/grpctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIdentityController serves IdentityBrokerService with canned responses.
type fakeIdentityController struct {
	aegisv1.UnimplementedIdentityBrokerServiceServer

	gotReq *aegisv1.ResolveAccountRequest
	resp   *aegisv1.ResolveAccountResponse
	err    error
}

func (f *fakeIdentityController) SD() *grpc.ServiceDesc {
	return &aegisv1.IdentityBrokerService_ServiceDesc
}

func (f *fakeIdentityController) ResolveAccount(_ context.Context, req *aegisv1.ResolveAccountRequest) (*aegisv1.ResolveAccountResponse, error) {
	f.gotReq = req
	return f.resp, f.err
}

func TestIdentityResolveAccount_MapsFields(t *testing.T) {
	ctrl := &fakeIdentityController{resp: &aegisv1.ResolveAccountResponse{
		AccountId: "acc-1", Email: "a@b.com", DisplayName: "A", Created: true,
	}}
	conn := grpctest.NewServer(t, ctrl)

	got, err := client.NewIdentityGRPCClient(conn).ResolveAccount(context.Background(), "realm", "google", "tok")
	require.NoError(t, err)
	assert.Equal(t, "acc-1", got.AccountID)
	assert.Equal(t, "a@b.com", got.Email)
	assert.True(t, got.Created)
	require.NotNil(t, ctrl.gotReq)
	assert.Equal(t, "realm", ctrl.gotReq.GetRealmId())
	assert.Equal(t, "google", ctrl.gotReq.GetIdpName())
	assert.Equal(t, "tok", ctrl.gotReq.GetToken())
}

func TestIdentityResolveAccount_MapsGRPCError(t *testing.T) {
	ctrl := &fakeIdentityController{err: status.Error(codes.Unauthenticated, "bad token")}
	conn := grpctest.NewServer(t, ctrl)

	_, err := client.NewIdentityGRPCClient(conn).ResolveAccount(context.Background(), "realm", "google", "tok")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

// fakeAuthorizerController serves AuthorizerService with canned responses.
type fakeAuthorizerController struct {
	aegisv1.UnimplementedAuthorizerServiceServer

	gotCheck *aegisv1.CheckRequest
	allowed  bool
	err      error
}

func (f *fakeAuthorizerController) SD() *grpc.ServiceDesc {
	return &aegisv1.AuthorizerService_ServiceDesc
}

func (f *fakeAuthorizerController) Check(_ context.Context, req *aegisv1.CheckRequest) (*aegisv1.CheckResponse, error) {
	f.gotCheck = req
	if f.err != nil {
		return nil, f.err
	}
	return &aegisv1.CheckResponse{Allowed: f.allowed}, nil
}

func (f *fakeAuthorizerController) BatchCheck(_ context.Context, req *aegisv1.BatchCheckRequest) (*aegisv1.BatchCheckResponse, error) {
	decisions := make([]*aegisv1.PermissionDecision, 0, len(req.GetChecks()))
	for _, check := range req.GetChecks() {
		decisions = append(decisions, &aegisv1.PermissionDecision{
			ResourceId:   check.GetResourceId(),
			PermissionId: check.GetPermissionId(),
			Allowed:      f.allowed,
		})
	}
	return &aegisv1.BatchCheckResponse{Decisions: decisions}, nil
}

func (f *fakeAuthorizerController) ListAccessible(_ context.Context, _ *aegisv1.ListAccessibleRequest) (*aegisv1.ListAccessibleResponse, error) {
	return &aegisv1.ListAccessibleResponse{ResourceIds: []string{"res-1", "res-2"}}, nil
}

func (f *fakeAuthorizerController) Version(_ context.Context, _ *aegisv1.VersionRequest) (*aegisv1.VersionResponse, error) {
	return &aegisv1.VersionResponse{WriteVersion: 7, ProjectionVersion: 5}, nil
}

func TestAuthorizerCheck_RoundTrips(t *testing.T) {
	ctrl := &fakeAuthorizerController{allowed: true}
	conn := grpctest.NewServer(t, ctrl)
	authorizer := client.NewAuthorizerGRPCClient(conn)

	allowed, err := authorizer.Check(context.Background(), client.Check{
		AccountID: "acc-1", ResourceID: "res-1", PermissionID: "doc.read", MinVersion: 3,
	})
	require.NoError(t, err)
	assert.True(t, allowed)
	require.NotNil(t, ctrl.gotCheck)
	assert.Equal(t, "acc-1", ctrl.gotCheck.GetAccountId())
	assert.Equal(t, "res-1", ctrl.gotCheck.GetResourceId())
	assert.Equal(t, "doc.read", ctrl.gotCheck.GetPermissionId())
	assert.Equal(t, int64(3), ctrl.gotCheck.GetMinVersion())
}

func TestAuthorizerCheck_MapsPreconditionError(t *testing.T) {
	ctrl := &fakeAuthorizerController{err: status.Error(codes.FailedPrecondition, "projection is stale")}
	conn := grpctest.NewServer(t, ctrl)

	_, err := client.NewAuthorizerGRPCClient(conn).Check(context.Background(), client.Check{
		AccountID: "acc-1", ResourceID: "res-1", PermissionID: "doc.read",
	})
	require.Error(t, err)
}

func TestAuthorizerBatchCheckListVersion(t *testing.T) {
	ctrl := &fakeAuthorizerController{allowed: true}
	conn := grpctest.NewServer(t, ctrl)
	authorizer := client.NewAuthorizerGRPCClient(conn)
	ctx := context.Background()

	decisions, err := authorizer.BatchCheck(ctx, client.BatchCheck{
		AccountID: "acc-1",
		Checks: []client.PermissionCheck{
			{ResourceID: "res-1", PermissionID: "doc.read"},
			{ResourceID: "res-2", PermissionID: "doc.write"},
		},
	})
	require.NoError(t, err)
	require.Len(t, decisions, 2)
	assert.Equal(t, "res-1", decisions[0].ResourceID)
	assert.True(t, decisions[0].Allowed)

	ids, err := authorizer.ListAccessible(ctx, client.ListAccessible{AccountID: "acc-1", PermissionID: "doc.read"})
	require.NoError(t, err)
	assert.Equal(t, []string{"res-1", "res-2"}, ids)

	version, err := authorizer.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(7), version.WriteVersion)
	assert.Equal(t, int64(5), version.ProjectionVersion)
}
