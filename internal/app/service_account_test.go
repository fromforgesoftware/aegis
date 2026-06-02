package app_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

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

func newServiceAccountUsecase(t *testing.T) (*apptest.ServiceAccountRepository, *apptest.AccountRepository, *apptest.TokenIssuer, app.ServiceAccountUsecase) {
	repo := apptest.NewServiceAccountRepository(t)
	accounts := apptest.NewAccountRepository(t)
	tokens := apptest.NewTokenIssuer(t)
	uc := app.NewServiceAccountUsecase(repo, accounts, tokens, persistencetest.NewTransactioner())
	return repo, accounts, tokens, uc
}

func TestServiceAccountCreate_MintsSERVICEAccountAndSecret(t *testing.T) {
	repo, accounts, _, uc := newServiceAccountUsecase(t)

	accounts.EXPECT().Create(mock.Anything, mock.MatchedBy(func(a domain.Account) bool {
		return a.AccountType() == domain.AccountTypeService && a.Status() == domain.AccountStatusEnabled
	})).Return(internaltest.NewAccount(internaltest.WithAccountID("acc-svc")), nil)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(sa domain.ServiceAccount) bool {
		return sa.ID() == "acc-svc" && sa.Name() == "ci-bot"
	}), mock.MatchedBy(func(hash string) bool { return hash != "" })).
		Return(domain.NewServiceAccount("r", "ci-bot", "client-x", domain.WithServiceAccountID("acc-svc")), nil)

	creds, err := uc.Create(context.Background(), "r", "ci-bot", []string{"audit:read"})
	require.NoError(t, err)
	assert.NotEmpty(t, creds.ClientID)
	assert.NotEmpty(t, creds.ClientSecret, "raw secret is surfaced once")
	assert.Equal(t, "acc-svc", creds.ServiceAccount.ID())
}

func TestServiceAccountCreate_RejectsMissingName(t *testing.T) {
	_, _, _, uc := newServiceAccountUsecase(t)
	_, err := uc.Create(context.Background(), "r", "", nil)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestServiceAccountIssueToken_SubjectsTokenToAccount(t *testing.T) {
	repo, _, tokens, uc := newServiceAccountUsecase(t)

	// A known service account whose secret hashes to the stored value.
	raw, hash := "s3cr3t-raw", sha256Hex("s3cr3t-raw")
	sa := domain.NewServiceAccount("r", "ci-bot", "client-x",
		domain.WithServiceAccountID("acc-svc"), domain.WithServiceAccountScopes([]string{"audit:read"}))
	repo.EXPECT().GetByClientID(mock.Anything, "r", "client-x").Return(sa, hash, nil)

	tokens.EXPECT().MintAccessToken(mock.Anything, "r", mock.MatchedBy(func(in app.AccessTokenInput) bool {
		return in.Subject == "acc-svc" && in.ClientID == "client-x"
	})).Return("token-abc", int64(3600), nil)
	repo.EXPECT().TouchLastUsed(mock.Anything, "acc-svc", mock.Anything).Return(nil)

	resp, err := uc.IssueToken(context.Background(), "r", "https://iss/realms/r", "client-x", raw)
	require.NoError(t, err)
	assert.Equal(t, "token-abc", resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)
}

func TestServiceAccountIssueToken_RejectsWrongSecret(t *testing.T) {
	repo, _, _, uc := newServiceAccountUsecase(t)
	sa := domain.NewServiceAccount("r", "ci-bot", "client-x", domain.WithServiceAccountID("acc-svc"))
	repo.EXPECT().GetByClientID(mock.Anything, "r", "client-x").Return(sa, sha256Hex("right"), nil)

	_, err := uc.IssueToken(context.Background(), "r", "iss", "client-x", "wrong")
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

func TestServiceAccountIssueToken_UnknownClient(t *testing.T) {
	repo, _, _, uc := newServiceAccountUsecase(t)
	repo.EXPECT().GetByClientID(mock.Anything, "r", "nope").Return(nil, "", apierrors.NotFound("service account", "nope"))

	_, err := uc.IssueToken(context.Background(), "r", "iss", "nope", "x")
	assert.True(t, apierrors.Is(err, apierrors.CodeUnauthenticated))
}

// sha256Hex mirrors the usecase's hashToken so a test can compute the stored
// secret hash for a known raw secret.
func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
