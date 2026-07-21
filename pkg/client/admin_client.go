package client

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// Health is the service's liveness snapshot.
type Health struct {
	Status  string
	Version string
}

// AdminAPI is the operational surface.
type AdminAPI interface {
	Healthz(ctx context.Context) (Health, error)
}

// ------------------------------------------------------------ GRPC

type adminGRPCClient struct {
	healthzEndpoint transport.Endpoint[struct{}, Health]
}

func NewAdminGRPCClient(conn kitgrpc.Conn) *adminGRPCClient {
	return &adminGRPCClient{
		healthzEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AdminService_ServiceDesc, "Healthz",
			encodeHealthzRequest, decodeHealthzResponse, kitgrpc.ClientAuthMiddleware(),
		),
	}
}

func (c *adminGRPCClient) Healthz(ctx context.Context) (Health, error) {
	health, err := kitgrpc.Call(ctx, c.healthzEndpoint, struct{}{})
	if err != nil {
		return Health{}, apierrors.FromGRPCError(err)
	}
	return health, nil
}

func encodeHealthzRequest(_ context.Context, _ struct{}) (*aegisv1.HealthzRequest, error) {
	return &aegisv1.HealthzRequest{}, nil
}

func decodeHealthzResponse(_ context.Context, resp *aegisv1.HealthzResponse) (Health, error) {
	return Health{Status: resp.GetStatus(), Version: resp.GetVersion()}, nil
}
