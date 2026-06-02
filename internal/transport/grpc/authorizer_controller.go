package grpc

import (
	"context"

	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

type authorizerController struct {
	authz app.AuthorizationUsecase
}

// NewAuthorizerController exposes the hot-path authorization reads over gRPC
// for S2S callers that gate requests on Aegis permissions.
func NewAuthorizerController(authz app.AuthorizationUsecase) kitgrpc.Controller {
	return &authorizerController{authz: authz}
}

func (c *authorizerController) SD() kitgrpc.ServiceDesc {
	return &aegisv1.AuthorizerService_ServiceDesc
}

func (c *authorizerController) Check(ctx context.Context, req *aegisv1.CheckRequest) (*aegisv1.CheckResponse, error) {
	allowed, err := c.authz.Check(ctx, req.GetAccountId(), req.GetResourceId(), req.GetPermissionId(), req.GetMinVersion())
	if err != nil {
		return nil, err
	}
	return &aegisv1.CheckResponse{Allowed: allowed}, nil
}

func (c *authorizerController) Version(ctx context.Context, _ *aegisv1.VersionRequest) (*aegisv1.VersionResponse, error) {
	write, projection, err := c.authz.Version(ctx)
	if err != nil {
		return nil, err
	}
	return &aegisv1.VersionResponse{WriteVersion: write, ProjectionVersion: projection}, nil
}

func (c *authorizerController) BatchCheck(ctx context.Context, req *aegisv1.BatchCheckRequest) (*aegisv1.BatchCheckResponse, error) {
	checks := make([]domain.PermissionCheck, 0, len(req.GetChecks()))
	for _, ch := range req.GetChecks() {
		checks = append(checks, domain.PermissionCheck{
			ResourceID:   ch.GetResourceId(),
			PermissionID: ch.GetPermissionId(),
		})
	}
	decisions, err := c.authz.BatchCheck(ctx, req.GetAccountId(), checks, req.GetMinVersion())
	if err != nil {
		return nil, err
	}
	resp := &aegisv1.BatchCheckResponse{Decisions: make([]*aegisv1.PermissionDecision, 0, len(decisions))}
	for _, d := range decisions {
		resp.Decisions = append(resp.Decisions, &aegisv1.PermissionDecision{
			ResourceId:   d.ResourceID,
			PermissionId: d.PermissionID,
			Allowed:      d.Allowed,
		})
	}
	return resp, nil
}

func (c *authorizerController) ListAccessible(ctx context.Context, req *aegisv1.ListAccessibleRequest) (*aegisv1.ListAccessibleResponse, error) {
	ids, err := c.authz.ListAccessible(ctx, req.GetAccountId(), req.GetPermissionId(), req.GetMinVersion())
	if err != nil {
		return nil, err
	}
	return &aegisv1.ListAccessibleResponse{ResourceIds: ids}, nil
}
