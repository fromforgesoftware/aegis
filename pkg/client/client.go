// Package client is the consumer-facing SDK for Aegis's low-latency S2S
// surfaces. Edge services dial Aegis once and resolve upstream identity tokens
// to account IDs on the request hot path; the middleware subpackage wraps this
// into drop-in gRPC/HTTP interceptors.
package client

import (
	"context"

	"google.golang.org/grpc"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// ResolvedAccount is the identity an upstream token maps to.
type ResolvedAccount struct {
	AccountID    string
	Email        string
	DisplayName  string
	Created      bool
	LinkRequired bool
}

// Client wraps Aegis's gRPC surfaces with kit error mapping.
type Client struct {
	identity aegisv1.IdentityBrokerServiceClient
}

// New builds a client over an established connection to Aegis.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{identity: aegisv1.NewIdentityBrokerServiceClient(conn)}
}

// NewFromIdentityClient is the seam tests use to inject a fake gRPC client.
func NewFromIdentityClient(identity aegisv1.IdentityBrokerServiceClient) *Client {
	return &Client{identity: identity}
}

// ResolveAccount verifies the upstream token against the realm's IdP and
// returns the Aegis account it maps to, JIT-provisioning on first contact.
func (c *Client) ResolveAccount(ctx context.Context, realmID, idpName, token string) (ResolvedAccount, error) {
	resp, err := c.identity.ResolveAccount(ctx, &aegisv1.ResolveAccountRequest{
		RealmId: realmID,
		IdpName: idpName,
		Token:   token,
	})
	if err != nil {
		return ResolvedAccount{}, apierrors.FromGRPCError(err)
	}
	return ResolvedAccount{
		AccountID:    resp.GetAccountId(),
		Email:        resp.GetEmail(),
		DisplayName:  resp.GetDisplayName(),
		Created:      resp.GetCreated(),
		LinkRequired: resp.GetLinkRequired(),
	}, nil
}
