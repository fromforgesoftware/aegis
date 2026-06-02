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
	"github.com/fromforgesoftware/aegis/internal/domain"
	aegisgrpc "github.com/fromforgesoftware/aegis/internal/transport/grpc"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

func TestAuthxController_Register_MapsRequestAndAccount(t *testing.T) {
	uc := apptest.NewAuthxUsecase(t)
	uc.EXPECT().
		Register(mock.Anything, app.RegisterInput{
			RealmID: "r", Email: "a@b.com", Password: "password123", DisplayName: "A",
		}).
		Return(domain.NewAccount("r", "a@b.com", "A",
			domain.WithAccountID("acc-1"), domain.WithAccountType(domain.AccountTypeUser)), nil)

	ctrl := aegisgrpc.NewAuthxController(uc, apptest.NewVerificationUsecase(t), apptest.NewPasswordResetUsecase(t)).(aegisv1.AuthxServiceServer)
	resp, err := ctrl.Register(context.Background(), &aegisv1.RegisterRequest{
		RealmId: "r", Email: "a@b.com", Password: "password123", DisplayName: "A",
	})
	require.NoError(t, err)
	assert.Equal(t, "acc-1", resp.GetAccount().GetId())
	assert.Equal(t, "a@b.com", resp.GetAccount().GetEmail())
	assert.Equal(t, "USER", resp.GetAccount().GetType())
	assert.Equal(t, "ENABLED", resp.GetAccount().GetStatus())
}

func TestAuthxController_Login_MapsAccount(t *testing.T) {
	uc := apptest.NewAuthxUsecase(t)
	uc.EXPECT().
		Login(mock.Anything, app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "password123"}).
		Return(domain.NewAccount("r", "a@b.com", "A", domain.WithAccountID("acc-1")), nil)

	ctrl := aegisgrpc.NewAuthxController(uc, apptest.NewVerificationUsecase(t), apptest.NewPasswordResetUsecase(t)).(aegisv1.AuthxServiceServer)
	resp, err := ctrl.Login(context.Background(), &aegisv1.LoginRequest{
		RealmId: "r", Email: "a@b.com", Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "acc-1", resp.GetAccount().GetId())
}

func TestAuthxController_Login_ErrorPassthrough(t *testing.T) {
	uc := apptest.NewAuthxUsecase(t)
	uc.EXPECT().Login(mock.Anything, app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "x"}).
		Return(nil, apierrors.Unauthenticated("invalid credentials"))

	ctrl := aegisgrpc.NewAuthxController(uc, apptest.NewVerificationUsecase(t), apptest.NewPasswordResetUsecase(t)).(aegisv1.AuthxServiceServer)
	_, err := ctrl.Login(context.Background(), &aegisv1.LoginRequest{RealmId: "r", Email: "a@b.com", Password: "x"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}
