package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor resolves the bearer token in the incoming metadata's
// "authorization" key to an account and injects it into the handler context.
// Requests to skipped methods pass through untouched.
func UnaryServerInterceptor(r Resolver, opts ...Option) grpc.UnaryServerInterceptor {
	cfg := newConfig(opts...)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.skip[info.FullMethod] {
			return handler(ctx, req)
		}
		token, ok := tokenFromMetadata(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing identity token")
		}
		acct, err := r.Resolve(ctx, cfg.realmID, cfg.idpName, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return handler(WithAccountID(ctx, acct.AccountID), req)
	}
}

func tokenFromMetadata(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", false
	}
	return bearerToken(vals[0])
}
