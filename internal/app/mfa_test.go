package app_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func newMFAUsecase(t *testing.T) (
	*apptest.MFAEnrollmentRepository,
	*apptest.RecoveryCodeRepository,
	*apptest.StepUpTokenRepository,
	app.KeyCipher,
	app.MFAUsecase,
) {
	enrollments := apptest.NewMFAEnrollmentRepository(t)
	recovery := apptest.NewRecoveryCodeRepository(t)
	stepup := apptest.NewStepUpTokenRepository(t)
	cipher := testKeyCipher(t)
	uc := app.NewMFAUsecase(enrollments, recovery, stepup, cipher, persistencetest.NewTransactioner())
	return enrollments, recovery, stepup, cipher, uc
}

// sealedEnrollment builds an enrollment carrying the given secret sealed with
// the same cipher the usecase uses, so VerifyTOTP can open it.
func sealedEnrollment(t *testing.T, cipher app.KeyCipher, accountID, secret string) domain.MFAEnrollment {
	sealed, err := cipher.Seal([]byte(secret))
	require.NoError(t, err)
	return domain.NewMFAEnrollment(accountID, domain.MFAFactorTOTP,
		domain.WithMFAEnrollmentSecret(base64.StdEncoding.EncodeToString(sealed)))
}

func TestMFAEnrollTOTP_ReturnsSecretAndURI(t *testing.T) {
	enrollments, _, _, _, uc := newMFAUsecase(t)
	enrollments.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil, nil)

	secret, uri, err := uc.EnrollTOTP(context.Background(), "acc-1", "Aegis", "user@x.com")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, uri, "otpauth://totp/")
}

func TestMFAVerifyTOTP_ValidCode(t *testing.T) {
	enrollments, _, _, cipher, uc := newMFAUsecase(t)
	secret, err := cryptox.GenerateTOTPSecret()
	require.NoError(t, err)
	enrollments.EXPECT().GetByAccountFactor(mock.Anything, "acc-1", domain.MFAFactorTOTP).
		Return(sealedEnrollment(t, cipher, "acc-1", secret), nil)

	code, err := cryptox.GenerateTOTPCode(secret, time.Now())
	require.NoError(t, err)
	ok, err := uc.VerifyTOTP(context.Background(), "acc-1", code)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMFAVerifyTOTP_BadCode(t *testing.T) {
	enrollments, _, _, cipher, uc := newMFAUsecase(t)
	secret, err := cryptox.GenerateTOTPSecret()
	require.NoError(t, err)
	enrollments.EXPECT().GetByAccountFactor(mock.Anything, "acc-1", domain.MFAFactorTOTP).
		Return(sealedEnrollment(t, cipher, "acc-1", secret), nil)

	ok, err := uc.VerifyTOTP(context.Background(), "acc-1", "000000")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMFAConfirmTOTP_ConfirmsAndIssuesRecoveryCodes(t *testing.T) {
	enrollments, recovery, _, cipher, uc := newMFAUsecase(t)
	secret, err := cryptox.GenerateTOTPSecret()
	require.NoError(t, err)
	enrollments.EXPECT().GetByAccountFactor(mock.Anything, "acc-1", domain.MFAFactorTOTP).
		Return(sealedEnrollment(t, cipher, "acc-1", secret), nil)
	enrollments.EXPECT().Confirm(mock.Anything, "acc-1", domain.MFAFactorTOTP, mock.Anything).Return(nil)
	recovery.EXPECT().DeleteByAccount(mock.Anything, "acc-1").Return(nil)
	recovery.EXPECT().CreateMany(mock.Anything, "acc-1", mock.MatchedBy(func(h []string) bool { return len(h) == 10 })).Return(nil)

	code, err := cryptox.GenerateTOTPCode(secret, time.Now())
	require.NoError(t, err)
	codes, err := uc.ConfirmTOTP(context.Background(), "acc-1", code)
	require.NoError(t, err)
	assert.Len(t, codes, 10)
}

func TestMFAStepUp_TOTPMintsToken(t *testing.T) {
	enrollments, _, stepup, cipher, uc := newMFAUsecase(t)
	secret, err := cryptox.GenerateTOTPSecret()
	require.NoError(t, err)
	enrollments.EXPECT().GetByAccountFactor(mock.Anything, "acc-1", domain.MFAFactorTOTP).
		Return(sealedEnrollment(t, cipher, "acc-1", secret), nil)
	stepup.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	code, err := cryptox.GenerateTOTPCode(secret, time.Now())
	require.NoError(t, err)
	token, acr, expiresAt, err := uc.StepUp(context.Background(), "acc-1", domain.MFAFactorTOTP, code)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, "aal2", acr)
	assert.True(t, expiresAt.After(time.Now()))
}

func TestMFAStepUp_BadProofRejected(t *testing.T) {
	enrollments, _, _, cipher, uc := newMFAUsecase(t)
	secret, err := cryptox.GenerateTOTPSecret()
	require.NoError(t, err)
	enrollments.EXPECT().GetByAccountFactor(mock.Anything, "acc-1", domain.MFAFactorTOTP).
		Return(sealedEnrollment(t, cipher, "acc-1", secret), nil)

	_, _, _, err = uc.StepUp(context.Background(), "acc-1", domain.MFAFactorTOTP, "000000")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestMFAVerifyRecoveryCode_Consumes(t *testing.T) {
	_, recovery, _, _, uc := newMFAUsecase(t)
	recovery.EXPECT().Consume(mock.Anything, "acc-1", mock.Anything, mock.Anything).Return(true, nil)

	ok, err := uc.VerifyRecoveryCode(context.Background(), "acc-1", "deadbeefdeadbeef")
	require.NoError(t, err)
	assert.True(t, ok)
}
