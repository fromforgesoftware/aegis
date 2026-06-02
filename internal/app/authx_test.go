package app_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestRegister_Validation(t *testing.T) {
	cases := []struct {
		name string
		in   app.RegisterInput
		// loadsPolicy is true once validation reaches the password check
		// (realm + email already passed), so the policy mock must expect it.
		loadsPolicy bool
	}{
		{"empty realm", app.RegisterInput{RealmID: "", Email: "a@b.com", Password: "password123"}, false},
		{"empty email", app.RegisterInput{RealmID: "r", Email: "", Password: "password123"}, false},
		{"short password", app.RegisterInput{RealmID: "r", Email: "a@b.com", Password: "short"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Validation fails before any account/hasher call; the mocks
			// (constructed with t) assert no unexpected calls happen.
			policies := apptest.NewPasswordPolicyRepository(t)
			if tc.loadsPolicy {
				policies.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.DefaultPasswordPolicy(), nil)
			}
			uc := app.NewAuthxUsecase(
				apptest.NewAccountRepository(t),
				apptest.NewCredentialRepository(t),
				policies,
				apptest.NewPasswordHasher(t),
				persistencetest.NewTransactioner(),
			)
			_, err := uc.Register(context.Background(), tc.in)
			require.Error(t, err)
			assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument), "want INVALID_ARGUMENT, got %v", err)
		})
	}
}

func TestRegister_Duplicate(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	existing := internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
		internaltest.WithAccountRealmID("r"), internaltest.WithAccountEmail("a@b.com"))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(existing, nil)
	policies := apptest.NewPasswordPolicyRepository(t)
	policies.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.DefaultPasswordPolicy(), nil)

	uc := app.NewAuthxUsecase(accounts, apptest.NewCredentialRepository(t), policies,
		apptest.NewPasswordHasher(t), persistencetest.NewTransactioner())
	_, err := uc.Register(context.Background(), app.RegisterInput{RealmID: "r", Email: "a@b.com", Password: "password123"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeAlreadyExists), "want ALREADY_EXISTS, got %v", err)
}

func TestRegister_Success(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	creds := apptest.NewCredentialRepository(t)
	hasher := apptest.NewPasswordHasher(t)
	policies := apptest.NewPasswordPolicyRepository(t)

	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("account", "a@b.com"))
	policies.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.DefaultPasswordPolicy(), nil)
	hasher.EXPECT().Hash("password123").Return(app.HashedPassword{Encoded: "enc", Algo: "argon2id"}, nil)

	// The account handed to Create carries the normalized email, realm and
	// profile from the input — matched precisely rather than mock.Anything.
	want := internaltest.NewAccount(internaltest.WithAccountRealmID("r"),
		internaltest.WithAccountEmail("a@b.com"), internaltest.WithAccountDisplayName("A"))
	created := internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
		internaltest.WithAccountRealmID("r"), internaltest.WithAccountEmail("a@b.com"),
		internaltest.WithAccountDisplayName("A"))
	accounts.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchAccount(want))).Return(created, nil)
	creds.EXPECT().SetPassword(mock.Anything, "acc-1", mock.Anything).Return(nil)

	uc := app.NewAuthxUsecase(accounts, creds, policies, hasher, persistencetest.NewTransactioner())
	got, err := uc.Register(context.Background(), app.RegisterInput{
		RealmID: "r", Email: "a@b.com", Password: "password123", DisplayName: "A",
	})
	require.NoError(t, err)
	assert.Equal(t, "acc-1", got.ID())
}

func TestLogin_UnknownAccount(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("account", "a@b.com"))

	uc := app.NewAuthxUsecase(accounts, apptest.NewCredentialRepository(t),
		apptest.NewPasswordPolicyRepository(t), apptest.NewPasswordHasher(t), persistencetest.NewTransactioner())
	_, err := uc.Login(context.Background(), app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "password123"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated), "want UNAUTHENTICATED, got %v", err)
}

func TestLogin_Disabled(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	disabled := internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
		internaltest.WithAccountStatus(domain.AccountStatusDisabled))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(disabled, nil)

	uc := app.NewAuthxUsecase(accounts, apptest.NewCredentialRepository(t),
		apptest.NewPasswordPolicyRepository(t), apptest.NewPasswordHasher(t), persistencetest.NewTransactioner())
	_, err := uc.Login(context.Background(), app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "password123"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestLogin_WrongPassword(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	creds := apptest.NewCredentialRepository(t)
	hasher := apptest.NewPasswordHasher(t)

	acc := internaltest.NewAccount(internaltest.WithAccountID("acc-1")) // ENABLED by default
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(acc, nil)
	creds.EXPECT().GetPasswordHash(mock.Anything, "acc-1").Return("enc", nil)
	hasher.EXPECT().Verify("wrongpass", "enc").Return(false, nil)
	// A wrong password records the failed attempt (increments the counter).
	accounts.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Account{acc}, nil)

	uc := app.NewAuthxUsecase(accounts, creds, apptest.NewPasswordPolicyRepository(t), hasher, persistencetest.NewTransactioner())
	_, err := uc.Login(context.Background(), app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "wrongpass"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestLogin_Locked(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	until := time.Now().Add(time.Hour)
	locked := internaltest.NewAccount(internaltest.WithAccountID("acc-1"),
		internaltest.WithAccountLockedUntil(&until))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(locked, nil)

	// A locked account is rejected before any credential check (no
	// GetPasswordHash / Verify / Patch calls — the mocks would fail otherwise).
	uc := app.NewAuthxUsecase(accounts, apptest.NewCredentialRepository(t),
		apptest.NewPasswordPolicyRepository(t), apptest.NewPasswordHasher(t), persistencetest.NewTransactioner())
	_, err := uc.Login(context.Background(), app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "password123"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeRateLimited), "want RATE_LIMITED, got %v", err)
}

func TestLogin_Success(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	creds := apptest.NewCredentialRepository(t)
	hasher := apptest.NewPasswordHasher(t)

	acc := internaltest.NewAccount(internaltest.WithAccountID("acc-1"))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(acc, nil)
	creds.EXPECT().GetPasswordHash(mock.Anything, "acc-1").Return("enc", nil)
	hasher.EXPECT().Verify("password123", "enc").Return(true, nil)
	accounts.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Account{acc}, nil)

	uc := app.NewAuthxUsecase(accounts, creds, apptest.NewPasswordPolicyRepository(t), hasher, persistencetest.NewTransactioner())
	got, err := uc.Login(context.Background(), app.LoginInput{RealmID: "r", Email: "a@b.com", Password: "password123"})
	require.NoError(t, err)
	assert.Equal(t, "acc-1", got.ID())
}

func TestLogin_RiskDenyBlocks(t *testing.T) {
	accounts := apptest.NewAccountRepository(t)
	creds := apptest.NewCredentialRepository(t)
	hasher := apptest.NewPasswordHasher(t)
	risk := apptest.NewRiskUsecase(t)

	acc := internaltest.NewAccount(internaltest.WithAccountID("acc-1"), internaltest.WithAccountRealmID("r"))
	accounts.EXPECT().Get(mock.Anything, mock.Anything).Return(acc, nil)
	creds.EXPECT().GetPasswordHash(mock.Anything, "acc-1").Return("enc", nil)
	hasher.EXPECT().Verify("password123", "enc").Return(true, nil)
	// Credentials are valid, but risk denies — no success Patch is expected.
	risk.EXPECT().Assess(mock.Anything, mock.MatchedBy(func(in app.RiskInput) bool {
		return in.AccountID == "acc-1" && in.IP == "9.9.9.9" && in.Succeeded
	})).Return(domain.RiskAssessment{Decision: domain.RiskDeny}, nil)

	uc := app.NewAuthxUsecase(accounts, creds, apptest.NewPasswordPolicyRepository(t), hasher,
		persistencetest.NewTransactioner(), app.WithRisk(risk))
	_, err := uc.Login(context.Background(), app.LoginInput{
		RealmID: "r", Email: "a@b.com", Password: "password123", IP: "9.9.9.9",
	})
	assert.True(t, apierrors.Is(err, apierrors.CodeForbidden), "risk DENY blocks login, got %v", err)
}
