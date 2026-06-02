package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence"

	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

const (
	recoveryCodeCount = 10
	stepUpTTL         = 5 * time.Minute
	stepUpACR         = "aal2"
)

// MFAEnrollmentRepository persists per-account factor enrollments.
type MFAEnrollmentRepository interface {
	Upsert(ctx context.Context, e domain.MFAEnrollment) (domain.MFAEnrollment, error)
	GetByAccountFactor(ctx context.Context, accountID string, factor domain.MFAFactor) (domain.MFAEnrollment, error)
	Confirm(ctx context.Context, accountID string, factor domain.MFAFactor, at time.Time) error
}

// RecoveryCodeRepository persists hashed one-time recovery codes.
type RecoveryCodeRepository interface {
	DeleteByAccount(ctx context.Context, accountID string) error
	CreateMany(ctx context.Context, accountID string, codeHashes []string) error
	Consume(ctx context.Context, accountID, codeHash string, at time.Time) (bool, error)
}

// StepUpTokenRepository persists short-lived re-auth proofs.
type StepUpTokenRepository interface {
	Create(ctx context.Context, t domain.StepUpToken) error
	Verify(ctx context.Context, id string, now time.Time) (domain.StepUpToken, error)
}

// MFAUsecase enrolls and verifies second factors and mints step-up tokens.
// EnrollTOTP returns the shared secret once for the authenticator app; the
// secret is sealed at rest. ConfirmTOTP turns enrollment on and hands back the
// recovery codes (also shown once).
type MFAUsecase interface {
	EnrollTOTP(ctx context.Context, accountID, issuer, accountLabel string) (secret, provisioningURI string, err error)
	ConfirmTOTP(ctx context.Context, accountID, code string) (recoveryCodes []string, err error)
	VerifyTOTP(ctx context.Context, accountID, code string) (bool, error)
	RegenerateRecoveryCodes(ctx context.Context, accountID string) ([]string, error)
	VerifyRecoveryCode(ctx context.Context, accountID, code string) (bool, error)
	StepUp(ctx context.Context, accountID string, factor domain.MFAFactor, proof string) (token, acr string, expiresAt time.Time, err error)
	VerifyStepUp(ctx context.Context, token string) (domain.StepUpToken, error)
}

type mfaUsecase struct {
	enrollments MFAEnrollmentRepository
	recovery    RecoveryCodeRepository
	stepup      StepUpTokenRepository
	cipher      KeyCipher
	tx          persistence.Transactioner
	now         func() time.Time
}

func NewMFAUsecase(
	enrollments MFAEnrollmentRepository,
	recovery RecoveryCodeRepository,
	stepup StepUpTokenRepository,
	cipher KeyCipher,
	tx persistence.Transactioner,
) MFAUsecase {
	return &mfaUsecase{
		enrollments: enrollments,
		recovery:    recovery,
		stepup:      stepup,
		cipher:      cipher,
		tx:          tx,
		now:         time.Now,
	}
}

func (uc *mfaUsecase) EnrollTOTP(ctx context.Context, accountID, issuer, accountLabel string) (string, string, error) {
	if accountID == "" {
		return "", "", apierrors.InvalidArgument("account id is required")
	}
	secret, err := cryptox.GenerateTOTPSecret()
	if err != nil {
		return "", "", apierrors.InternalError("could not generate secret")
	}
	sealed, err := uc.cipher.Seal([]byte(secret))
	if err != nil {
		return "", "", apierrors.InternalError("could not seal secret")
	}
	enrollment := domain.NewMFAEnrollment(accountID, domain.MFAFactorTOTP,
		domain.WithMFAEnrollmentSecret(base64.StdEncoding.EncodeToString(sealed)))
	if _, err := uc.enrollments.Upsert(ctx, enrollment); err != nil {
		return "", "", err
	}
	return secret, cryptox.TOTPProvisioningURI(secret, issuer, accountLabel), nil
}

func (uc *mfaUsecase) ConfirmTOTP(ctx context.Context, accountID, code string) ([]string, error) {
	ok, err := uc.VerifyTOTP(ctx, accountID, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apierrors.InvalidArgument("invalid code")
	}
	if err := uc.enrollments.Confirm(ctx, accountID, domain.MFAFactorTOTP, uc.now()); err != nil {
		return nil, err
	}
	return uc.RegenerateRecoveryCodes(ctx, accountID)
}

func (uc *mfaUsecase) VerifyTOTP(ctx context.Context, accountID, code string) (bool, error) {
	enrollment, err := uc.enrollments.GetByAccountFactor(ctx, accountID, domain.MFAFactorTOTP)
	if err != nil {
		return false, err
	}
	sealed, err := base64.StdEncoding.DecodeString(enrollment.Secret())
	if err != nil {
		return false, apierrors.InternalError("corrupt secret")
	}
	secret, err := uc.cipher.Open(sealed)
	if err != nil {
		return false, apierrors.InternalError("could not open secret")
	}
	return cryptox.ValidateTOTP(string(secret), code, uc.now()), nil
}

func (uc *mfaUsecase) RegenerateRecoveryCodes(ctx context.Context, accountID string) ([]string, error) {
	codes := make([]string, 0, recoveryCodeCount)
	hashes := make([]string, 0, recoveryCodeCount)
	for range recoveryCodeCount {
		code, err := randomCode()
		if err != nil {
			return nil, apierrors.InternalError("could not generate recovery code")
		}
		codes = append(codes, code)
		hashes = append(hashes, hashCode(code))
	}
	err := uc.tx.Exec(ctx, func(ctx context.Context) error {
		if err := uc.recovery.DeleteByAccount(ctx, accountID); err != nil {
			return err
		}
		return uc.recovery.CreateMany(ctx, accountID, hashes)
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (uc *mfaUsecase) VerifyRecoveryCode(ctx context.Context, accountID, code string) (bool, error) {
	return uc.recovery.Consume(ctx, accountID, hashCode(code), uc.now())
}

func (uc *mfaUsecase) StepUp(ctx context.Context, accountID string, factor domain.MFAFactor, proof string) (string, string, time.Time, error) {
	if accountID == "" {
		return "", "", time.Time{}, apierrors.InvalidArgument("account id is required")
	}
	var (
		ok  bool
		err error
	)
	switch factor {
	case domain.MFAFactorTOTP:
		ok, err = uc.VerifyTOTP(ctx, accountID, proof)
	case domain.MFAFactorRecovery:
		ok, err = uc.VerifyRecoveryCode(ctx, accountID, proof)
	default:
		return "", "", time.Time{}, apierrors.InvalidArgument("unsupported step-up factor")
	}
	if err != nil {
		return "", "", time.Time{}, err
	}
	if !ok {
		return "", "", time.Time{}, apierrors.Unauthenticated("step-up verification failed")
	}

	token, err := randomToken()
	if err != nil {
		return "", "", time.Time{}, apierrors.InternalError("could not mint step-up token")
	}
	expiresAt := uc.now().Add(stepUpTTL)
	if err := uc.stepup.Create(ctx, domain.NewStepUpToken(hashStepUpToken(token), accountID, factor, stepUpACR, expiresAt)); err != nil {
		return "", "", time.Time{}, err
	}
	return token, stepUpACR, expiresAt, nil
}

func (uc *mfaUsecase) VerifyStepUp(ctx context.Context, token string) (domain.StepUpToken, error) {
	if token == "" {
		return nil, apierrors.InvalidArgument("token is required")
	}
	return uc.stepup.Verify(ctx, hashStepUpToken(token), uc.now())
}

func randomCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func hashStepUpToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
