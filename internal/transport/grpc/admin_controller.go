// Package grpc holds Aegis's gRPC controllers. Each controller
// implements kitgrpc.Controller (SD + the service methods) and is
// registered with the kit's gRPC gateway via grpc.NewFxController.
package grpc

import (
	"context"

	"github.com/fromforgesoftware/go-kit/app"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

type adminController struct {
	version string
}

// NewAdminController builds the AdminService controller. The version is
// pulled from the app.Info value the kit provides to the fx graph.
func NewAdminController(info app.Info) kitgrpc.Controller {
	return &adminController{version: info.Version}
}

func (c *adminController) SD() kitgrpc.ServiceDesc {
	return &aegisv1.AdminService_ServiceDesc
}

func (c *adminController) Healthz(_ context.Context, _ *aegisv1.HealthzRequest) (*aegisv1.HealthzResponse, error) {
	return &aegisv1.HealthzResponse{Status: "SERVING", Version: c.version}, nil
}
