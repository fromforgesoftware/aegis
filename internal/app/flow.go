package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/search"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// flowTTL is how long an interactive flow stays submittable after it starts.
const flowTTL = 30 * time.Minute

// FlowRepository persists interactive auth flows via the kit generics.
type FlowRepository interface {
	repository.Creator[domain.Flow]
	repository.Getter[domain.Flow]
	repository.Patcher[domain.Flow]
}

// SubmitFlowInput advances a flow with the client's field values.
type SubmitFlowInput struct {
	FlowID  string
	Payload map[string]string
}

// FlowUsecase is the API-first foundation for interactive auth. Create
// (start) and Get satisfy the kit's resource generics so the JSON:API
// handlers map straight onto them; Submit is the operation that advances a
// flow by orchestrating the underlying auth usecases.
type FlowUsecase interface {
	repository.Creator[domain.Flow]
	repository.Getter[domain.Flow]
	Submit(ctx context.Context, in SubmitFlowInput) (domain.Flow, error)
}

type flowUsecase struct {
	flows         FlowRepository
	authx         AuthxUsecase
	verification  VerificationUsecase
	passwordReset PasswordResetUsecase
}

func NewFlowUsecase(
	flows FlowRepository,
	authx AuthxUsecase,
	verification VerificationUsecase,
	passwordReset PasswordResetUsecase,
) FlowUsecase {
	return &flowUsecase{flows: flows, authx: authx, verification: verification, passwordReset: passwordReset}
}

// Create starts a flow. realmId + flowType come off the request; expiry
// and state are server-authoritative.
func (uc *flowUsecase) Create(ctx context.Context, in domain.Flow) (domain.Flow, error) {
	if in.RealmID() == "" {
		return nil, apierrors.InvalidArgument("realm_id is required")
	}
	if !in.FlowType().Valid() {
		return nil, apierrors.InvalidArgument("invalid flow type")
	}
	f := domain.NewFlow(in.RealmID(), in.FlowType(), time.Now().UTC().Add(flowTTL))
	return uc.flows.Create(ctx, f)
}

func (uc *flowUsecase) Get(ctx context.Context, opts ...search.Option) (domain.Flow, error) {
	return uc.flows.Get(ctx, opts...)
}

func (uc *flowUsecase) Submit(ctx context.Context, in SubmitFlowInput) (domain.Flow, error) {
	f, err := uc.flows.Get(ctx, byID(in.FlowID))
	if err != nil {
		return nil, err
	}
	if f.State() != domain.FlowStatePending {
		return nil, apierrors.InvalidArgument("flow is already completed")
	}
	if domain.FlowExpired(f, time.Now().UTC()) {
		return nil, apierrors.InvalidArgument("flow has expired")
	}
	if err := validateFlowPayload(f.FlowType(), in.Payload); err != nil {
		return nil, err
	}

	// A failed submission leaves the flow PENDING so the client can retry
	// (until expiry); only a successful step completes it.
	resultAccountID, err := uc.dispatch(ctx, f, in.Payload)
	if err != nil {
		return nil, err
	}

	patch := map[string]any{fields.State: string(domain.FlowStateCompleted)}
	if resultAccountID != "" {
		patch[fields.ResultAccountID] = resultAccountID
	}
	updated, err := uc.flows.Patch(ctx,
		repository.PatchSearchOpts(byID(f.ID())),
		repository.WithPatchFields(patch),
	)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, apierrors.InternalError("flow completion patched no rows")
	}
	return updated[0], nil
}

// dispatch runs the flow type's underlying operation and returns the
// resolved account id (empty for recovery/verification).
func (uc *flowUsecase) dispatch(ctx context.Context, f domain.Flow, payload map[string]string) (string, error) {
	switch f.FlowType() {
	case domain.FlowTypeLogin:
		acc, err := uc.authx.Login(ctx, LoginInput{RealmID: f.RealmID(), Email: payload["email"], Password: payload["password"]})
		if err != nil {
			return "", err
		}
		return acc.ID(), nil
	case domain.FlowTypeRegistration:
		acc, err := uc.authx.Register(ctx, RegisterInput{
			RealmID: f.RealmID(), Email: payload["email"], Password: payload["password"], DisplayName: payload["displayName"],
		})
		if err != nil {
			return "", err
		}
		return acc.ID(), nil
	case domain.FlowTypeRecovery:
		return "", uc.passwordReset.RequestPasswordReset(ctx, f.RealmID(), payload["email"])
	case domain.FlowTypeVerification:
		return "", uc.verification.VerifyEmail(ctx, payload["token"])
	}
	return "", apierrors.InvalidArgument("unsupported flow type")
}

func validateFlowPayload(t domain.FlowType, payload map[string]string) error {
	for _, f := range domain.RequiredFields(t) {
		if f.Required && strings.TrimSpace(payload[f.Name]) == "" {
			return apierrors.InvalidArgument(fmt.Sprintf("%s is required", f.Name))
		}
	}
	return nil
}
