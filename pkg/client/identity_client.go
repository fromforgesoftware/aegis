package client

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// IdentityAPI brokers upstream identity tokens: it verifies the token against
// the realm's IdP and returns the Aegis account it maps to, JIT-provisioning
// on first contact. It satisfies AccountResolver, so it can be wrapped by
// CachingResolver and the middleware subpackage.
type IdentityAPI interface {
	ResolveAccount(ctx context.Context, realmID, idpName, token string) (ResolvedAccount, error)
}

// ------------------------------------------------------------ GRPC

type resolveAccountRequest struct {
	realmID string
	idpName string
	token   string
}

type identityGRPCClient struct {
	resolveAccountEndpoint transport.Endpoint[resolveAccountRequest, ResolvedAccount]
}

func NewIdentityGRPCClient(conn kitgrpc.Conn) *identityGRPCClient {
	return &identityGRPCClient{
		resolveAccountEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.IdentityBrokerService_ServiceDesc, "ResolveAccount",
			encodeResolveAccountRequest, decodeResolveAccountResponse, kitgrpc.ClientAuthMiddleware(),
		),
	}
}

func (c *identityGRPCClient) ResolveAccount(ctx context.Context, realmID, idpName, token string) (ResolvedAccount, error) {
	account, err := kitgrpc.Call(ctx, c.resolveAccountEndpoint, resolveAccountRequest{
		realmID: realmID, idpName: idpName, token: token,
	})
	if err != nil {
		return ResolvedAccount{}, apierrors.FromGRPCError(err)
	}
	return account, nil
}

func encodeResolveAccountRequest(_ context.Context, req resolveAccountRequest) (*aegisv1.ResolveAccountRequest, error) {
	return &aegisv1.ResolveAccountRequest{
		RealmId: req.realmID,
		IdpName: req.idpName,
		Token:   req.token,
	}, nil
}

func decodeResolveAccountResponse(_ context.Context, resp *aegisv1.ResolveAccountResponse) (ResolvedAccount, error) {
	return ResolvedAccount{
		AccountID:    resp.GetAccountId(),
		Email:        resp.GetEmail(),
		DisplayName:  resp.GetDisplayName(),
		Created:      resp.GetCreated(),
		LinkRequired: resp.GetLinkRequired(),
	}, nil
}
