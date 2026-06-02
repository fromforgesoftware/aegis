package grpc

import (
	"context"

	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

type mfaController struct {
	mfa app.MFAUsecase
}

// NewMFAController exposes step-up over gRPC for S2S callers gating sensitive
// operations on fresh re-auth.
func NewMFAController(mfa app.MFAUsecase) kitgrpc.Controller {
	return &mfaController{mfa: mfa}
}

func (c *mfaController) SD() kitgrpc.ServiceDesc {
	return &aegisv1.MFAService_ServiceDesc
}

func (c *mfaController) StepUp(ctx context.Context, req *aegisv1.StepUpRequest) (*aegisv1.StepUpResponse, error) {
	token, acr, expiresAt, err := c.mfa.StepUp(ctx, req.GetAccountId(), domain.MFAFactor(req.GetFactor()), req.GetProof())
	if err != nil {
		return nil, err
	}
	return &aegisv1.StepUpResponse{Token: token, Acr: acr, ExpiresAt: expiresAt.Unix()}, nil
}

func (c *mfaController) VerifyStepUp(ctx context.Context, req *aegisv1.VerifyStepUpRequest) (*aegisv1.VerifyStepUpResponse, error) {
	tok, err := c.mfa.VerifyStepUp(ctx, req.GetToken())
	if err != nil {
		return &aegisv1.VerifyStepUpResponse{Valid: false}, nil
	}
	return &aegisv1.VerifyStepUpResponse{Valid: true, AccountId: tok.AccountID(), Acr: tok.ACR()}, nil
}
