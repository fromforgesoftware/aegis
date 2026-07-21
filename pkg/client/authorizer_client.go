package client

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// Check asks whether AccountID holds PermissionID on ResourceID. MinVersion
// (optional) asserts read-after-write freshness: pass the write version read
// after a mutation and the check fails with a precondition error while the
// projection is still behind it.
type Check struct {
	AccountID    string
	ResourceID   string
	PermissionID string
	MinVersion   int64
}

// PermissionCheck is one (resource, permission) pair inside a batch.
type PermissionCheck struct {
	ResourceID   string
	PermissionID string
}

// BatchCheck answers many permission checks for one account in a single call.
type BatchCheck struct {
	AccountID  string
	Checks     []PermissionCheck
	MinVersion int64
}

// PermissionDecision is one answer inside a batch response.
type PermissionDecision struct {
	ResourceID   string
	PermissionID string
	Allowed      bool
}

// ListAccessible asks for every resource id AccountID holds PermissionID on —
// the query backing permission-filtered list endpoints.
type ListAccessible struct {
	AccountID    string
	PermissionID string
	MinVersion   int64
}

// AuthzVersion is the read-after-write counter snapshot: writeVersion is the
// latest mutation, projectionVersion is what Check currently reflects.
type AuthzVersion struct {
	WriteVersion      int64
	ProjectionVersion int64
}

// AuthorizerAPI is the ReBAC decision surface (gRPC hot path). A false
// decision is not an error — only transport/backend failures are.
type AuthorizerAPI interface {
	Check(ctx context.Context, check Check) (bool, error)
	BatchCheck(ctx context.Context, batch BatchCheck) ([]PermissionDecision, error)
	ListAccessible(ctx context.Context, list ListAccessible) ([]string, error)
	Version(ctx context.Context) (AuthzVersion, error)
}

// ------------------------------------------------------------ GRPC

type authorizerGRPCClient struct {
	checkEndpoint          transport.Endpoint[Check, bool]
	batchCheckEndpoint     transport.Endpoint[BatchCheck, []PermissionDecision]
	listAccessibleEndpoint transport.Endpoint[ListAccessible, []string]
	versionEndpoint        transport.Endpoint[struct{}, AuthzVersion]
}

func NewAuthorizerGRPCClient(conn kitgrpc.Conn) *authorizerGRPCClient {
	return &authorizerGRPCClient{
		checkEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthorizerService_ServiceDesc, "Check",
			encodeCheckRequest, decodeCheckResponse, kitgrpc.ClientAuthMiddleware(),
		),
		batchCheckEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthorizerService_ServiceDesc, "BatchCheck",
			encodeBatchCheckRequest, decodeBatchCheckResponse, kitgrpc.ClientAuthMiddleware(),
		),
		listAccessibleEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthorizerService_ServiceDesc, "ListAccessible",
			encodeListAccessibleRequest, decodeListAccessibleResponse, kitgrpc.ClientAuthMiddleware(),
		),
		versionEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthorizerService_ServiceDesc, "Version",
			encodeVersionRequest, decodeVersionResponse, kitgrpc.ClientAuthMiddleware(),
		),
	}
}

func (c *authorizerGRPCClient) Check(ctx context.Context, check Check) (bool, error) {
	allowed, err := kitgrpc.Call(ctx, c.checkEndpoint, check)
	if err != nil {
		return false, apierrors.FromGRPCError(err)
	}
	return allowed, nil
}

func (c *authorizerGRPCClient) BatchCheck(ctx context.Context, batch BatchCheck) ([]PermissionDecision, error) {
	decisions, err := kitgrpc.Call(ctx, c.batchCheckEndpoint, batch)
	if err != nil {
		return nil, apierrors.FromGRPCError(err)
	}
	return decisions, nil
}

func (c *authorizerGRPCClient) ListAccessible(ctx context.Context, list ListAccessible) ([]string, error) {
	ids, err := kitgrpc.Call(ctx, c.listAccessibleEndpoint, list)
	if err != nil {
		return nil, apierrors.FromGRPCError(err)
	}
	return ids, nil
}

func (c *authorizerGRPCClient) Version(ctx context.Context) (AuthzVersion, error) {
	version, err := kitgrpc.Call(ctx, c.versionEndpoint, struct{}{})
	if err != nil {
		return AuthzVersion{}, apierrors.FromGRPCError(err)
	}
	return version, nil
}

func encodeCheckRequest(_ context.Context, check Check) (*aegisv1.CheckRequest, error) {
	return &aegisv1.CheckRequest{
		AccountId:    check.AccountID,
		ResourceId:   check.ResourceID,
		PermissionId: check.PermissionID,
		MinVersion:   check.MinVersion,
	}, nil
}

func decodeCheckResponse(_ context.Context, resp *aegisv1.CheckResponse) (bool, error) {
	return resp.GetAllowed(), nil
}

func encodeBatchCheckRequest(_ context.Context, batch BatchCheck) (*aegisv1.BatchCheckRequest, error) {
	checks := make([]*aegisv1.PermissionCheck, 0, len(batch.Checks))
	for _, check := range batch.Checks {
		checks = append(checks, &aegisv1.PermissionCheck{
			ResourceId:   check.ResourceID,
			PermissionId: check.PermissionID,
		})
	}
	return &aegisv1.BatchCheckRequest{
		AccountId:  batch.AccountID,
		Checks:     checks,
		MinVersion: batch.MinVersion,
	}, nil
}

func decodeBatchCheckResponse(_ context.Context, resp *aegisv1.BatchCheckResponse) ([]PermissionDecision, error) {
	decisions := make([]PermissionDecision, 0, len(resp.GetDecisions()))
	for _, d := range resp.GetDecisions() {
		decisions = append(decisions, PermissionDecision{
			ResourceID:   d.GetResourceId(),
			PermissionID: d.GetPermissionId(),
			Allowed:      d.GetAllowed(),
		})
	}
	return decisions, nil
}

func encodeListAccessibleRequest(_ context.Context, list ListAccessible) (*aegisv1.ListAccessibleRequest, error) {
	return &aegisv1.ListAccessibleRequest{
		AccountId:    list.AccountID,
		PermissionId: list.PermissionID,
		MinVersion:   list.MinVersion,
	}, nil
}

func decodeListAccessibleResponse(_ context.Context, resp *aegisv1.ListAccessibleResponse) ([]string, error) {
	return resp.GetResourceIds(), nil
}

func encodeVersionRequest(_ context.Context, _ struct{}) (*aegisv1.VersionRequest, error) {
	return &aegisv1.VersionRequest{}, nil
}

func decodeVersionResponse(_ context.Context, resp *aegisv1.VersionResponse) (AuthzVersion, error) {
	return AuthzVersion{
		WriteVersion:      resp.GetWriteVersion(),
		ProjectionVersion: resp.GetProjectionVersion(),
	}, nil
}
