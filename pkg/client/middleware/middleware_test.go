package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fromforgesoftware/aegis/pkg/client"
	"github.com/fromforgesoftware/aegis/pkg/client/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	account client.ResolvedAccount
	err     error
}

func (f fakeResolver) Resolve(context.Context, string, string, string) (client.ResolvedAccount, error) {
	return f.account, f.err
}

func grpcCtx(token string) context.Context {
	return metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer "+token))
}

func TestGRPCInterceptor_InjectsAccount(t *testing.T) {
	interceptor := middleware.UnaryServerInterceptor(fakeResolver{account: client.ResolvedAccount{AccountID: "acc-1"}})
	var seen string
	_, err := interceptor(grpcCtx("tok"), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(ctx context.Context, _ any) (any, error) {
			seen, _ = middleware.AccountIDFromContext(ctx)
			return nil, nil
		})
	require.NoError(t, err)
	assert.Equal(t, "acc-1", seen)
}

func TestGRPCInterceptor_MissingTokenUnauthenticated(t *testing.T) {
	interceptor := middleware.UnaryServerInterceptor(fakeResolver{})
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, any) (any, error) { return nil, nil })
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGRPCInterceptor_ResolveFailureUnauthenticated(t *testing.T) {
	interceptor := middleware.UnaryServerInterceptor(fakeResolver{err: errors.New("bad token")})
	_, err := interceptor(grpcCtx("tok"), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, any) (any, error) { return nil, nil })
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGRPCInterceptor_SkipMethodBypasses(t *testing.T) {
	interceptor := middleware.UnaryServerInterceptor(fakeResolver{}, middleware.Skip("/grpc.health.v1.Health/Check"))
	called := false
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.NoError(t, err)
	assert.True(t, called, "skipped method runs without a token")
}

func TestHTTPMiddleware_InjectsAccount(t *testing.T) {
	var seen string
	h := middleware.HTTP(fakeResolver{account: client.ResolvedAccount{AccountID: "acc-1"}})(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen, _ = middleware.AccountIDFromContext(r.Context())
		}))

	req := httptest.NewRequest(http.MethodGet, "/api/things", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "acc-1", seen)
}

func TestHTTPMiddleware_MissingTokenUnauthorized(t *testing.T) {
	h := middleware.HTTP(fakeResolver{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/things", http.NoBody))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHTTPMiddleware_SkipPathBypasses(t *testing.T) {
	called := false
	h := middleware.HTTP(fakeResolver{}, middleware.Skip("/healthz"))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}
