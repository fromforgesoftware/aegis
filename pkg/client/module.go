package client

import (
	"fmt"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config wires a client to one Aegis deployment. GRPCAddr backs the hot-path
// APIs (Authorizer, Identity, ...); HTTPURL backs the JSON:API admin surface.
// Either may be empty — the APIs of an unconfigured transport are nil on the
// returned Client. The realm is never configured here: it travels per call.
type Config struct {
	GRPCAddr      string // e.g. "aegis:9090"
	HTTPURL       string // e.g. "http://aegis:80"
	GatewaySecret string // FORGE_GATEWAY_SECRET; empty ⇒ Aegis /api is ungated
	ServiceName   string // gateway-token subject label; defaults to "aegis-client"
}

// Dial builds a Client from plain configuration, returning a closer for the
// underlying gRPC connection.
func Dial(cfg Config) (Client, func() error, error) {
	if cfg.GRPCAddr == "" && cfg.HTTPURL == "" {
		return nil, nil, fmt.Errorf("aegis client: at least one of GRPCAddr or HTTPURL is required")
	}

	var (
		admin      AdminAPI
		authx      AuthxAPI
		oauth      OAuthAPI
		identity   IdentityAPI
		authorizer AuthorizerAPI
		mfa        MFAAPI
	)
	closer := func() error { return nil }
	if cfg.GRPCAddr != "" {
		conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, fmt.Errorf("aegis client: dial %s: %w", cfg.GRPCAddr, err)
		}
		closer = conn.Close
		admin = NewAdminGRPCClient(conn)
		authx = NewAuthxGRPCClient(conn)
		oauth = NewOAuthGRPCClient(conn)
		identity = NewIdentityGRPCClient(conn)
		authorizer = NewAuthorizerGRPCClient(conn)
		mfa = NewMFAGRPCClient(conn)
	}

	var (
		resource   ResourceAPI
		binding    BindingAPI
		role       RoleAPI
		permission PermissionAPI
		group      GroupAPI
		authzAdmin AuthorizationAdminAPI
	)
	if cfg.HTTPURL != "" {
		rest, err := newRESTClientFromConfig(cfg)
		if err != nil {
			_ = closer()
			return nil, nil, err
		}
		resource = NewResourceHTTPClient(rest)
		binding = NewBindingHTTPClient(rest)
		role = NewRoleHTTPClient(rest)
		permission = NewPermissionHTTPClient(rest)
		group = NewGroupHTTPClient(rest)
		authzAdmin = NewAuthorizationAdminHTTPClient(rest)
	}

	return NewClient(
		admin, authx, oauth, identity, authorizer, mfa,
		resource, binding, role, permission, group, authzAdmin,
	), closer, nil
}

// NewRESTClient builds the shared HTTP transport the JSON:API surfaces run
// on. Exposed for consumers that wire APIs individually (fx).
func NewRESTClient(cfg Config) (*restClient, error) {
	return newRESTClientFromConfig(cfg)
}

func newRESTClientFromConfig(cfg Config) (*restClient, error) {
	if cfg.HTTPURL == "" {
		return nil, fmt.Errorf("aegis client: HTTPURL is required")
	}
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "aegis-client"
	}
	var issuer gatewayTokenIssuer
	if cfg.GatewaySecret != "" {
		hmacIssuer, err := jwt.NewHMACIssuer(cfg.GatewaySecret)
		if err != nil {
			return nil, fmt.Errorf("aegis client: gateway issuer: %w", err)
		}
		issuer = hmacIssuer
	}
	return newRESTClient(cfg.HTTPURL, issuer, serviceName), nil
}

// FxModule wires every Aegis API plus the aggregate Client into an fx app,
// sharing one gRPC connection across the gRPC APIs (legacy account-service
// client pattern). name namespaces the connection ("aegis" is fine). Both
// transports are required here — for a partial (gRPC-only or HTTP-only)
// client use Dial, or wire the individual API constructors yourself.
func FxModule(name string, cfg Config) fx.Option {
	if cfg.GRPCAddr == "" || cfg.HTTPURL == "" {
		return fx.Error(fmt.Errorf("aegis client: FxModule needs both GRPCAddr and HTTPURL (got %q, %q)", cfg.GRPCAddr, cfg.HTTPURL))
	}

	return fx.Module("aegis:"+name,
		fx.Supply(cfg),
		kitgrpc.FxClientModule(
			name, cfg.GRPCAddr,
			func(conn kitgrpc.Conn) AdminAPI { return NewAdminGRPCClient(conn) },
			kitgrpc.WithClientProviders(
				func(conn kitgrpc.Conn) AuthxAPI { return NewAuthxGRPCClient(conn) },
				func(conn kitgrpc.Conn) OAuthAPI { return NewOAuthGRPCClient(conn) },
				func(conn kitgrpc.Conn) IdentityAPI { return NewIdentityGRPCClient(conn) },
				func(conn kitgrpc.Conn) AuthorizerAPI { return NewAuthorizerGRPCClient(conn) },
				func(conn kitgrpc.Conn) MFAAPI { return NewMFAGRPCClient(conn) },
			),
		),
		fx.Provide(
			NewRESTClient,
			fx.Annotate(NewResourceHTTPClient, fx.As(new(ResourceAPI))),
			fx.Annotate(NewBindingHTTPClient, fx.As(new(BindingAPI))),
			fx.Annotate(NewRoleHTTPClient, fx.As(new(RoleAPI))),
			fx.Annotate(NewPermissionHTTPClient, fx.As(new(PermissionAPI))),
			fx.Annotate(NewGroupHTTPClient, fx.As(new(GroupAPI))),
			fx.Annotate(NewAuthorizationAdminHTTPClient, fx.As(new(AuthorizationAdminAPI))),
			fx.Annotate(NewClient, fx.As(new(Client))),
		),
	)
}
