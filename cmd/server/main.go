// Command server boots the Aegis identity + authorization service:
// the kit's REST gateway (OpenAPI 3.1) plus a gRPC server exposing
// AdminService. Later waves add the authx (authentication +
// authorization) and identity surfaces.
package main

import (
	"github.com/fromforgesoftware/go-kit/app"
	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormpg"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	"github.com/fromforgesoftware/aegis/internal"
)

// Config is supplied entirely by the environment (12-factor). In k8s the
// Helm chart is the source of truth (deploy/helm/values.yaml → ports +
// chart name); for local dev `make aegis-run` sets the same vars. The
// kit's logger/tracer require SVC_NAME and the REST module requires
// HTTP_ADDRESS, so the process fails fast if the runtime hasn't set them.
func main() {
	app.Run(
		app.WithName("aegis"),
		app.WithVersion(internal.Version),
		app.WithOpenAPI(
			openapi.SpecTitle("Aegis"),
			openapi.SpecVersion(internal.Version),
			openapi.SpecDescription("Forge identity + authorization service."),
		),
		gormpg.FxModule(),
		kitgrpc.FxModule(),
		internal.FxModule(),
	)
}
