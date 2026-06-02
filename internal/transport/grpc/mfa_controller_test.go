package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	aegisgrpc "github.com/fromforgesoftware/aegis/internal/transport/grpc"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

func TestMFAController_StepUp(t *testing.T) {
	uc := apptest.NewMFAUsecase(t)
	exp := time.Now().Add(5 * time.Minute)
	uc.EXPECT().StepUp(mock.Anything, "acc-1", domain.MFAFactorTOTP, "123456").
		Return("tok", "aal2", exp, nil)

	ctrl := aegisgrpc.NewMFAController(uc).(aegisv1.MFAServiceServer)
	resp, err := ctrl.StepUp(context.Background(), &aegisv1.StepUpRequest{
		AccountId: "acc-1", Factor: "TOTP", Proof: "123456",
	})
	require.NoError(t, err)
	assert.Equal(t, "tok", resp.GetToken())
	assert.Equal(t, "aal2", resp.GetAcr())
	assert.Equal(t, exp.Unix(), resp.GetExpiresAt())
}

func TestMFAController_VerifyStepUp_Valid(t *testing.T) {
	uc := apptest.NewMFAUsecase(t)
	uc.EXPECT().VerifyStepUp(mock.Anything, "tok").Return(
		domain.NewStepUpToken("id", "acc-1", domain.MFAFactorTOTP, "aal2", time.Now().Add(time.Minute)), nil)

	ctrl := aegisgrpc.NewMFAController(uc).(aegisv1.MFAServiceServer)
	resp, err := ctrl.VerifyStepUp(context.Background(), &aegisv1.VerifyStepUpRequest{Token: "tok"})
	require.NoError(t, err)
	assert.True(t, resp.GetValid())
	assert.Equal(t, "acc-1", resp.GetAccountId())
}

func TestMFAController_VerifyStepUp_InvalidReturnsFalse(t *testing.T) {
	uc := apptest.NewMFAUsecase(t)
	uc.EXPECT().VerifyStepUp(mock.Anything, "bad").Return(nil, errors.New("expired"))

	ctrl := aegisgrpc.NewMFAController(uc).(aegisv1.MFAServiceServer)
	resp, err := ctrl.VerifyStepUp(context.Background(), &aegisv1.VerifyStepUpRequest{Token: "bad"})
	require.NoError(t, err)
	assert.False(t, resp.GetValid())
}
