package client

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// RefreshGrant exchanges a refresh token for a new token pair.
type RefreshGrant struct {
	RealmID      string
	Issuer       string
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// Token is an issued OAuth2 token response.
type Token struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int64
	RefreshToken string
	IDToken      string
	Scope        string
}

// TokenIntrospection asks whether a token is active (RFC 7662).
type TokenIntrospection struct {
	RealmID       string
	ClientID      string
	ClientSecret  string
	Token         string
	TokenTypeHint string
}

// Introspection is the token's state and claims.
type Introspection struct {
	Active    bool
	Scope     string
	ClientID  string
	Subject   string
	TokenType string
	ExpiresAt int64
	IssuedAt  int64
	Audience  string
	Issuer    string
	OrgID     string
	OrgRole   string
}

// Revocation invalidates a token (RFC 7009).
type Revocation struct {
	RealmID       string
	ClientID      string
	ClientSecret  string
	Token         string
	TokenTypeHint string
}

// OAuthAPI is the OAuth2 token lifecycle surface for confidential clients.
type OAuthAPI interface {
	Refresh(ctx context.Context, grant RefreshGrant) (Token, error)
	Introspect(ctx context.Context, introspection TokenIntrospection) (Introspection, error)
	Revoke(ctx context.Context, revocation Revocation) error
}

// ------------------------------------------------------------ GRPC

type oauthGRPCClient struct {
	refreshEndpoint    transport.Endpoint[RefreshGrant, Token]
	introspectEndpoint transport.Endpoint[TokenIntrospection, Introspection]
	revokeEndpoint     transport.Endpoint[Revocation, struct{}]
}

func NewOAuthGRPCClient(conn kitgrpc.Conn) *oauthGRPCClient {
	return &oauthGRPCClient{
		refreshEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.OAuthService_ServiceDesc, "Refresh",
			encodeRefreshRequest, decodeTokenResponse, kitgrpc.ClientAuthMiddleware(),
		),
		introspectEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.OAuthService_ServiceDesc, "Introspect",
			encodeIntrospectRequest, decodeIntrospectResponse, kitgrpc.ClientAuthMiddleware(),
		),
		revokeEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.OAuthService_ServiceDesc, "Revoke",
			encodeRevokeRequest, decodeEmptyResponse[*aegisv1.RevokeResponse],
			kitgrpc.ClientAuthMiddleware(),
		),
	}
}

func (c *oauthGRPCClient) Refresh(ctx context.Context, grant RefreshGrant) (Token, error) {
	token, err := kitgrpc.Call(ctx, c.refreshEndpoint, grant)
	if err != nil {
		return Token{}, apierrors.FromGRPCError(err)
	}
	return token, nil
}

func (c *oauthGRPCClient) Introspect(ctx context.Context, introspection TokenIntrospection) (Introspection, error) {
	result, err := kitgrpc.Call(ctx, c.introspectEndpoint, introspection)
	if err != nil {
		return Introspection{}, apierrors.FromGRPCError(err)
	}
	return result, nil
}

func (c *oauthGRPCClient) Revoke(ctx context.Context, revocation Revocation) error {
	if _, err := kitgrpc.Call(ctx, c.revokeEndpoint, revocation); err != nil {
		return apierrors.FromGRPCError(err)
	}
	return nil
}

func encodeRefreshRequest(_ context.Context, grant RefreshGrant) (*aegisv1.RefreshRequest, error) {
	return &aegisv1.RefreshRequest{
		RealmId:      grant.RealmID,
		Issuer:       grant.Issuer,
		ClientId:     grant.ClientID,
		ClientSecret: grant.ClientSecret,
		RefreshToken: grant.RefreshToken,
	}, nil
}

func decodeTokenResponse(_ context.Context, resp *aegisv1.TokenResponse) (Token, error) {
	return Token{
		AccessToken:  resp.GetAccessToken(),
		TokenType:    resp.GetTokenType(),
		ExpiresIn:    resp.GetExpiresIn(),
		RefreshToken: resp.GetRefreshToken(),
		IDToken:      resp.GetIdToken(),
		Scope:        resp.GetScope(),
	}, nil
}

func encodeIntrospectRequest(_ context.Context, introspection TokenIntrospection) (*aegisv1.IntrospectRequest, error) {
	return &aegisv1.IntrospectRequest{
		RealmId:       introspection.RealmID,
		ClientId:      introspection.ClientID,
		ClientSecret:  introspection.ClientSecret,
		Token:         introspection.Token,
		TokenTypeHint: introspection.TokenTypeHint,
	}, nil
}

func decodeIntrospectResponse(_ context.Context, resp *aegisv1.IntrospectResponse) (Introspection, error) {
	return Introspection{
		Active:    resp.GetActive(),
		Scope:     resp.GetScope(),
		ClientID:  resp.GetClientId(),
		Subject:   resp.GetSub(),
		TokenType: resp.GetTokenType(),
		ExpiresAt: resp.GetExp(),
		IssuedAt:  resp.GetIat(),
		Audience:  resp.GetAud(),
		Issuer:    resp.GetIss(),
		OrgID:     resp.GetOrgId(),
		OrgRole:   resp.GetOrgRole(),
	}, nil
}

func encodeRevokeRequest(_ context.Context, revocation Revocation) (*aegisv1.RevokeRequest, error) {
	return &aegisv1.RevokeRequest{
		RealmId:       revocation.RealmID,
		ClientId:      revocation.ClientID,
		ClientSecret:  revocation.ClientSecret,
		Token:         revocation.Token,
		TokenTypeHint: revocation.TokenTypeHint,
	}, nil
}
