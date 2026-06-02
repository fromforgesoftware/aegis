package client_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
	"github.com/fromforgesoftware/aegis/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIdentityClient struct {
	resp *aegisv1.ResolveAccountResponse
	err  error
}

func (f *fakeIdentityClient) ResolveAccount(context.Context, *aegisv1.ResolveAccountRequest, ...grpc.CallOption) (*aegisv1.ResolveAccountResponse, error) {
	return f.resp, f.err
}

func TestClientResolveAccount_MapsFields(t *testing.T) {
	c := client.NewFromIdentityClient(&fakeIdentityClient{resp: &aegisv1.ResolveAccountResponse{
		AccountId: "acc-1", Email: "a@b.com", DisplayName: "A", Created: true,
	}})
	got, err := c.ResolveAccount(context.Background(), "realm", "google", "tok")
	require.NoError(t, err)
	assert.Equal(t, "acc-1", got.AccountID)
	assert.Equal(t, "a@b.com", got.Email)
	assert.True(t, got.Created)
}

func TestClientResolveAccount_MapsGRPCError(t *testing.T) {
	c := client.NewFromIdentityClient(&fakeIdentityClient{err: status.Error(codes.Unauthenticated, "bad token")})
	_, err := c.ResolveAccount(context.Background(), "realm", "google", "tok")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}
