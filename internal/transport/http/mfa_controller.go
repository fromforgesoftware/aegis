package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// MFAController exposes TOTP enrolment, verification, recovery codes, and the
// REST mirror of the gRPC step-up.
type MFAController struct {
	mfa app.MFAUsecase
}

func NewMFAController(mfa app.MFAUsecase) kitrest.Controller {
	return &MFAController{mfa: mfa}
}

func (c *MFAController) Routes(r kitrest.Router) {
	r.Route("/api/mfa", func(r kitrest.Router) {
		r.Post("/enroll", kitrest.NewJsonApiCommandHandler(
			c.enroll, decodeMFAEnroll, identityMFAEnrollDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Enrol TOTP"), openapi.Tags("mfa"), openapi.Errors(400)),
		))
		r.Post("/confirm", kitrest.NewJsonApiCommandHandler(
			c.confirm, decodeMFACode, identityRecoveryCodesDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Confirm TOTP enrolment"), openapi.Tags("mfa"), openapi.Errors(400)),
		))
		r.Post("/verify", kitrest.NewJsonApiCommandHandler(
			c.verify, decodeMFACode, identityVerificationDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Verify a TOTP code"), openapi.Tags("mfa"), openapi.Errors(400)),
		))
		r.Post("/recovery-codes", kitrest.NewJsonApiCommandHandler(
			c.regenerate, decodeMFAAccount, identityRecoveryCodesDTO,
			kitrest.HandlerWithOpenAPI(openapi.Summary("Regenerate recovery codes"), openapi.Tags("mfa"), openapi.Errors(400)),
		))
		r.Post("/step-up", kitrest.NewJsonApiCommandHandler(
			c.stepUp, decodeStepUp, identityStepUpDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Step up with a second factor"), openapi.Tags("mfa"), openapi.Errors(400, 401)),
		))
	})
}

func (c *MFAController) enroll(ctx context.Context, in api.MFAEnrollRequestDTO) (*api.MFAEnrollDTO, error) {
	issuer := in.RIssuer
	if issuer == "" {
		issuer = "Aegis"
	}
	secret, uri, err := c.mfa.EnrollTOTP(ctx, in.RAccountID, issuer, in.RAccountLabel)
	if err != nil {
		return nil, err
	}
	return api.MFAEnrollToDTO(secret, uri), nil
}

func (c *MFAController) confirm(ctx context.Context, in api.MFACodeRequestDTO) (*api.MFARecoveryCodesDTO, error) {
	codes, err := c.mfa.ConfirmTOTP(ctx, in.RAccountID, in.RCode)
	if err != nil {
		return nil, err
	}
	return api.MFARecoveryCodesToDTO(codes), nil
}

func (c *MFAController) verify(ctx context.Context, in api.MFACodeRequestDTO) (*api.MFAVerificationDTO, error) {
	ok, err := c.mfa.VerifyTOTP(ctx, in.RAccountID, in.RCode)
	if err != nil {
		return nil, err
	}
	return api.MFAVerificationToDTO(ok), nil
}

func (c *MFAController) regenerate(ctx context.Context, in api.MFACodeRequestDTO) (*api.MFARecoveryCodesDTO, error) {
	codes, err := c.mfa.RegenerateRecoveryCodes(ctx, in.RAccountID)
	if err != nil {
		return nil, err
	}
	return api.MFARecoveryCodesToDTO(codes), nil
}

func (c *MFAController) stepUp(ctx context.Context, in api.StepUpRequestDTO) (*api.StepUpDTO, error) {
	token, acr, expiresAt, err := c.mfa.StepUp(ctx, in.RAccountID, domain.MFAFactor(in.RFactor), in.RProof)
	if err != nil {
		return nil, err
	}
	return api.StepUpToDTO(token, acr, expiresAt), nil
}

func decodeMFAEnroll(req *http.Request) (api.MFAEnrollRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.MFAEnrollRequestDTO](req)
	if err != nil {
		return api.MFAEnrollRequestDTO{}, err
	}
	return *body, nil
}

func decodeMFACode(req *http.Request) (api.MFACodeRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.MFACodeRequestDTO](req)
	if err != nil {
		return api.MFACodeRequestDTO{}, err
	}
	return *body, nil
}

func decodeMFAAccount(req *http.Request) (api.MFACodeRequestDTO, error) {
	return decodeMFACode(req)
}

func decodeStepUp(req *http.Request) (api.StepUpRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.StepUpRequestDTO](req)
	if err != nil {
		return api.StepUpRequestDTO{}, err
	}
	return *body, nil
}

func identityMFAEnrollDTO(dto *api.MFAEnrollDTO) *api.MFAEnrollDTO                   { return dto }
func identityRecoveryCodesDTO(dto *api.MFARecoveryCodesDTO) *api.MFARecoveryCodesDTO { return dto }
func identityVerificationDTO(dto *api.MFAVerificationDTO) *api.MFAVerificationDTO    { return dto }
func identityStepUpDTO(dto *api.StepUpDTO) *api.StepUpDTO                            { return dto }
