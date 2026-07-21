package client

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// StepUpChallenge proves a second factor (TOTP or RECOVERY) for an account.
type StepUpChallenge struct {
	AccountID string
	Factor    string
	Proof     string
}

// StepUp is a short-lived elevated-auth token with its achieved ACR.
type StepUp struct {
	Token     string
	ACR       string
	ExpiresAt time.Time
}

// StepUpVerification is the outcome of validating a step-up token.
type StepUpVerification struct {
	Valid     bool
	AccountID string
	ACR       string
}

// MFAAPI is the step-up (elevated authentication) surface.
type MFAAPI interface {
	StepUp(ctx context.Context, challenge StepUpChallenge) (StepUp, error)
	VerifyStepUp(ctx context.Context, token string) (StepUpVerification, error)
}

// ------------------------------------------------------------ GRPC

type mfaGRPCClient struct {
	stepUpEndpoint       transport.Endpoint[StepUpChallenge, StepUp]
	verifyStepUpEndpoint transport.Endpoint[string, StepUpVerification]
}

func NewMFAGRPCClient(conn kitgrpc.Conn) *mfaGRPCClient {
	return &mfaGRPCClient{
		stepUpEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.MFAService_ServiceDesc, "StepUp",
			encodeStepUpRequest, decodeStepUpResponse, kitgrpc.ClientAuthMiddleware(),
		),
		verifyStepUpEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.MFAService_ServiceDesc, "VerifyStepUp",
			encodeVerifyStepUpRequest, decodeVerifyStepUpResponse, kitgrpc.ClientAuthMiddleware(),
		),
	}
}

func (c *mfaGRPCClient) StepUp(ctx context.Context, challenge StepUpChallenge) (StepUp, error) {
	stepUp, err := kitgrpc.Call(ctx, c.stepUpEndpoint, challenge)
	if err != nil {
		return StepUp{}, apierrors.FromGRPCError(err)
	}
	return stepUp, nil
}

func (c *mfaGRPCClient) VerifyStepUp(ctx context.Context, token string) (StepUpVerification, error) {
	verification, err := kitgrpc.Call(ctx, c.verifyStepUpEndpoint, token)
	if err != nil {
		return StepUpVerification{}, apierrors.FromGRPCError(err)
	}
	return verification, nil
}

func encodeStepUpRequest(_ context.Context, challenge StepUpChallenge) (*aegisv1.StepUpRequest, error) {
	return &aegisv1.StepUpRequest{
		AccountId: challenge.AccountID,
		Factor:    challenge.Factor,
		Proof:     challenge.Proof,
	}, nil
}

func decodeStepUpResponse(_ context.Context, resp *aegisv1.StepUpResponse) (StepUp, error) {
	return StepUp{
		Token:     resp.GetToken(),
		ACR:       resp.GetAcr(),
		ExpiresAt: time.Unix(resp.GetExpiresAt(), 0).UTC(),
	}, nil
}

func encodeVerifyStepUpRequest(_ context.Context, token string) (*aegisv1.VerifyStepUpRequest, error) {
	return &aegisv1.VerifyStepUpRequest{Token: token}, nil
}

func decodeVerifyStepUpResponse(_ context.Context, resp *aegisv1.VerifyStepUpResponse) (StepUpVerification, error) {
	return StepUpVerification{
		Valid:     resp.GetValid(),
		AccountID: resp.GetAccountId(),
		ACR:       resp.GetAcr(),
	}, nil
}
