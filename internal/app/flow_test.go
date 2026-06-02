package app_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newFlow(t *testing.T) (
	*apptest.FlowRepository,
	*apptest.AuthxUsecase,
	*apptest.VerificationUsecase,
	*apptest.PasswordResetUsecase,
	app.FlowUsecase,
) {
	flows := apptest.NewFlowRepository(t)
	authx := apptest.NewAuthxUsecase(t)
	verification := apptest.NewVerificationUsecase(t)
	passwordReset := apptest.NewPasswordResetUsecase(t)
	uc := app.NewFlowUsecase(flows, authx, verification, passwordReset)
	return flows, authx, verification, passwordReset, uc
}

func TestCreateFlow_Success(t *testing.T) {
	flows, _, _, _, uc := newFlow(t)
	// Create builds a server-authoritative flow from the realm + type; the
	// persisted flow matches those, with state PENDING.
	want := internaltest.NewFlow(internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeLogin))
	created := internaltest.NewFlow(internaltest.WithFlowID("flow-1"),
		internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeLogin))
	flows.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchFlow(want))).Return(created, nil)

	got, err := uc.Create(context.Background(),
		internaltest.NewFlow(internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeLogin)))
	require.NoError(t, err)
	assert.Equal(t, "flow-1", got.ID())
	assert.Equal(t, domain.FlowStatePending, got.State())
}

func TestCreateFlow_InvalidType(t *testing.T) {
	_, _, _, _, uc := newFlow(t)
	_, err := uc.Create(context.Background(), internaltest.NewFlow(internaltest.WithFlowType(domain.FlowType("bogus"))))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)
}

func TestCreateFlow_EmptyRealm(t *testing.T) {
	_, _, _, _, uc := newFlow(t)
	_, err := uc.Create(context.Background(), internaltest.NewFlow(internaltest.WithFlowRealmID("")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestSubmitFlow_LoginSuccess(t *testing.T) {
	flows, authx, _, _, uc := newFlow(t)
	pending := internaltest.NewFlow(internaltest.WithFlowID("flow-1"),
		internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeLogin))
	flows.EXPECT().Get(mock.Anything, mock.Anything).Return(pending, nil)
	authx.EXPECT().Login(mock.Anything, app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "password123"}).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1")), nil)
	completed := internaltest.NewFlow(internaltest.WithFlowID("flow-1"),
		internaltest.WithFlowState(domain.FlowStateCompleted), internaltest.WithFlowResultAccountID("acc-1"))
	flows.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Flow{completed}, nil)

	got, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com", "password": "password123"},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.FlowStateCompleted, got.State())
	assert.Equal(t, "acc-1", got.ResultAccountID())
}

func TestSubmitFlow_RegistrationSuccess(t *testing.T) {
	flows, authx, _, _, uc := newFlow(t)
	pending := internaltest.NewFlow(internaltest.WithFlowID("flow-1"),
		internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeRegistration))
	flows.EXPECT().Get(mock.Anything, mock.Anything).Return(pending, nil)
	authx.EXPECT().Register(mock.Anything, app.RegisterInput{RealmID: "r", Email: "a@b.com", Password: "password123", DisplayName: "A"}).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acc-1")), nil)
	flows.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything).
		Return([]domain.Flow{internaltest.NewFlow(internaltest.WithFlowState(domain.FlowStateCompleted))}, nil)

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com", "password": "password123", "displayName": "A"},
	})
	require.NoError(t, err)
}

func TestSubmitFlow_RecoverySuccess(t *testing.T) {
	flows, _, _, pr, uc := newFlow(t)
	pending := internaltest.NewFlow(internaltest.WithFlowID("flow-1"),
		internaltest.WithFlowRealmID("r"), internaltest.WithFlowType(domain.FlowTypeRecovery))
	flows.EXPECT().Get(mock.Anything, mock.Anything).Return(pending, nil)
	pr.EXPECT().RequestPasswordReset(mock.Anything, "r", "a@b.com").Return(nil)
	flows.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything).
		Return([]domain.Flow{internaltest.NewFlow(internaltest.WithFlowState(domain.FlowStateCompleted))}, nil)

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com"},
	})
	require.NoError(t, err)
}

func TestSubmitFlow_VerificationSuccess(t *testing.T) {
	flows, _, verification, _, uc := newFlow(t)
	pending := internaltest.NewFlow(internaltest.WithFlowID("flow-1"),
		internaltest.WithFlowType(domain.FlowTypeVerification))
	flows.EXPECT().Get(mock.Anything, mock.Anything).Return(pending, nil)
	verification.EXPECT().VerifyEmail(mock.Anything, "tok-123").Return(nil)
	flows.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything).
		Return([]domain.Flow{internaltest.NewFlow(internaltest.WithFlowState(domain.FlowStateCompleted))}, nil)

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"token": "tok-123"},
	})
	require.NoError(t, err)
}

func TestSubmitFlow_AlreadyCompleted(t *testing.T) {
	flows, _, _, _, uc := newFlow(t)
	// No dispatch/patch when the flow is already terminal.
	flows.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewFlow(internaltest.WithFlowState(domain.FlowStateCompleted)), nil)

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com", "password": "password123"},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)
}

func TestSubmitFlow_Expired(t *testing.T) {
	flows, _, _, _, uc := newFlow(t)
	flows.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewFlow(internaltest.WithFlowExpiresAt(time.Now().Add(-time.Minute))), nil)

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com", "password": "password123"},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestSubmitFlow_MissingRequiredField(t *testing.T) {
	flows, _, _, _, uc := newFlow(t)
	// Login flow missing the password → rejected before any dispatch.
	flows.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewFlow(internaltest.WithFlowType(domain.FlowTypeLogin)), nil)

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com"},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestSubmitFlow_LoginFailureStaysPending(t *testing.T) {
	flows, authx, _, _, uc := newFlow(t)
	flows.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewFlow(internaltest.WithFlowType(domain.FlowTypeLogin)), nil)
	authx.EXPECT().Login(mock.Anything, mock.Anything).Return(nil, apierrors.Unauthenticated("invalid credentials"))
	// No Patch: a failed submission must NOT complete the flow.

	_, err := uc.Submit(context.Background(), app.SubmitFlowInput{
		FlowID: "flow-1", Payload: map[string]string{"email": "a@b.com", "password": "wrong"},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "want UNAUTHENTICATED, got %v", err)
}
