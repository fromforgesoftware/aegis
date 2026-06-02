package grpc_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	aegisgrpc "github.com/fromforgesoftware/aegis/internal/transport/grpc"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

func newOAuthServer(t *testing.T, uc app.OAuthUsecase) aegisv1.OAuthServiceServer {
	return aegisgrpc.NewOAuthController(uc).(aegisv1.OAuthServiceServer)
}

func TestOAuthController_Refresh_MapsRequestAndResponse(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Refresh(mock.Anything, app.RefreshInput{
		RealmID: "r", Issuer: "https://auth/realms/r", ClientID: "web", RefreshToken: "rt",
	}).Return(app.TokenResponse{
		AccessToken: "at", TokenType: "Bearer", ExpiresIn: 3600, RefreshToken: "rt-2", Scope: "openid",
	}, nil)

	resp, err := newOAuthServer(t, uc).Refresh(context.Background(), &aegisv1.RefreshRequest{
		RealmId: "r", Issuer: "https://auth/realms/r", ClientId: "web", RefreshToken: "rt",
	})
	require.NoError(t, err)
	assert.Equal(t, "at", resp.GetAccessToken())
	assert.Equal(t, "rt-2", resp.GetRefreshToken())
	assert.Equal(t, int64(3600), resp.GetExpiresIn())
}

func TestOAuthController_Introspect_MapsClaims(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Introspect(mock.Anything, app.IntrospectInput{
		RealmID: "r", ClientID: "web", Token: "tok",
	}).Return(app.IntrospectionResult{
		Active: true, TokenType: "Bearer", Subject: "acc-1", Scope: "openid", ClientID: "web",
	}, nil)

	resp, err := newOAuthServer(t, uc).Introspect(context.Background(), &aegisv1.IntrospectRequest{
		RealmId: "r", ClientId: "web", Token: "tok",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetActive())
	assert.Equal(t, "acc-1", resp.GetSub())
	assert.Equal(t, "Bearer", resp.GetTokenType())
}

func TestOAuthController_Revoke_OK(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Revoke(mock.Anything, app.RevokeInput{RealmID: "r", ClientID: "web", Token: "tok"}).Return(nil)

	_, err := newOAuthServer(t, uc).Revoke(context.Background(), &aegisv1.RevokeRequest{
		RealmId: "r", ClientId: "web", Token: "tok",
	})
	require.NoError(t, err)
}

func TestOAuthController_Refresh_ErrorPassthrough(t *testing.T) {
	uc := apptest.NewOAuthUsecase(t)
	uc.EXPECT().Refresh(mock.Anything, app.RefreshInput{
		RealmID: "r", ClientID: "web", RefreshToken: "reused",
	}).Return(app.TokenResponse{}, apierrors.InvalidArgument("refresh token reuse detected"))

	_, err := newOAuthServer(t, uc).Refresh(context.Background(), &aegisv1.RefreshRequest{
		RealmId: "r", ClientId: "web", RefreshToken: "reused",
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
