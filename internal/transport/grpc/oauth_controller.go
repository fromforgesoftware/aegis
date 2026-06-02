package grpc

import (
	"context"

	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	"github.com/fromforgesoftware/aegis/internal/app"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

type oauthController struct {
	oauth app.OAuthUsecase
}

// NewOAuthController builds the OAuthService controller: the low-latency S2S
// mirror of the HTTP token/introspect/revoke endpoints. Returned errors are
// apierrors values the kit's gRPC layer maps to status codes.
func NewOAuthController(oauth app.OAuthUsecase) kitgrpc.Controller {
	return &oauthController{oauth: oauth}
}

func (c *oauthController) SD() kitgrpc.ServiceDesc {
	return &aegisv1.OAuthService_ServiceDesc
}

func (c *oauthController) Refresh(ctx context.Context, req *aegisv1.RefreshRequest) (*aegisv1.TokenResponse, error) {
	resp, err := c.oauth.Refresh(ctx, app.RefreshInput{
		RealmID:      req.GetRealmId(),
		Issuer:       req.GetIssuer(),
		ClientID:     req.GetClientId(),
		ClientSecret: req.GetClientSecret(),
		RefreshToken: req.GetRefreshToken(),
	})
	if err != nil {
		return nil, err
	}
	return &aegisv1.TokenResponse{
		AccessToken:  resp.AccessToken,
		TokenType:    resp.TokenType,
		ExpiresIn:    resp.ExpiresIn,
		RefreshToken: resp.RefreshToken,
		IdToken:      resp.IDToken,
		Scope:        resp.Scope,
	}, nil
}

func (c *oauthController) Introspect(ctx context.Context, req *aegisv1.IntrospectRequest) (*aegisv1.IntrospectResponse, error) {
	res, err := c.oauth.Introspect(ctx, app.IntrospectInput{
		RealmID:       req.GetRealmId(),
		ClientID:      req.GetClientId(),
		ClientSecret:  req.GetClientSecret(),
		Token:         req.GetToken(),
		TokenTypeHint: req.GetTokenTypeHint(),
	})
	if err != nil {
		return nil, err
	}
	return &aegisv1.IntrospectResponse{
		Active:    res.Active,
		Scope:     res.Scope,
		ClientId:  res.ClientID,
		Sub:       res.Subject,
		TokenType: res.TokenType,
		Exp:       res.Exp,
		Iat:       res.Iat,
		Aud:       res.Audience,
		Iss:       res.Issuer,
		OrgId:     res.OrgID,
		OrgRole:   res.OrgRole,
	}, nil
}

func (c *oauthController) Revoke(ctx context.Context, req *aegisv1.RevokeRequest) (*aegisv1.RevokeResponse, error) {
	if err := c.oauth.Revoke(ctx, app.RevokeInput{
		RealmID:       req.GetRealmId(),
		ClientID:      req.GetClientId(),
		ClientSecret:  req.GetClientSecret(),
		Token:         req.GetToken(),
		TokenTypeHint: req.GetTokenTypeHint(),
	}); err != nil {
		return nil, err
	}
	return &aegisv1.RevokeResponse{}, nil
}
