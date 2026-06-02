package app

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence"
	"github.com/google/uuid"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

const serviceAccountTokenTTL = time.Hour

// ServiceAccountRepository persists service accounts. The secret hash is
// written on Create and read back only by GetByClientID for authentication.
type ServiceAccountRepository interface {
	repository.Getter[domain.ServiceAccount]
	repository.Lister[domain.ServiceAccount]
	Create(ctx context.Context, sa domain.ServiceAccount, secretHash string) (domain.ServiceAccount, error)
	GetByClientID(ctx context.Context, realmID, clientID string) (domain.ServiceAccount, string, error)
	Delete(ctx context.Context, accountID string) error
	TouchLastUsed(ctx context.Context, accountID string, at time.Time) error
}

// ServiceAccountCredentials is the one-time create result: the persisted
// account plus the raw secret, surfaced once and never stored in the clear.
type ServiceAccountCredentials struct {
	ServiceAccount domain.ServiceAccount
	ClientID       string
	ClientSecret   string
}

// ServiceAccountUsecase manages machine identities: create (mints credentials),
// list, delete, and issue an access token via client_credentials. Issued tokens
// are subject'd to the SERVICE account id, so the platform's authz resolves
// against the account's grants — unlike a bare confidential-client token.
type ServiceAccountUsecase interface {
	repository.Getter[domain.ServiceAccount]
	repository.Lister[domain.ServiceAccount]
	Create(ctx context.Context, realmID, name string, scopes []string) (ServiceAccountCredentials, error)
	Delete(ctx context.Context, accountID string) error
	IssueToken(ctx context.Context, realmID, issuer, clientID, clientSecret string) (TokenResponse, error)
}

type serviceAccountUsecase struct {
	usecase.Getter[domain.ServiceAccount]
	usecase.Lister[domain.ServiceAccount]

	repo     ServiceAccountRepository
	accounts AccountRepository
	tokens   TokenIssuer
	tx       persistence.Transactioner
	now      func() time.Time
}

func NewServiceAccountUsecase(
	repo ServiceAccountRepository,
	accounts AccountRepository,
	tokens TokenIssuer,
	tx persistence.Transactioner,
) ServiceAccountUsecase {
	return &serviceAccountUsecase{
		Getter:   usecase.NewGetter(repo, domain.ResourceTypeServiceAccount),
		Lister:   usecase.NewLister[domain.ServiceAccount](repo),
		repo:     repo,
		accounts: accounts,
		tokens:   tokens,
		tx:       tx,
		now:      time.Now,
	}
}

func (uc *serviceAccountUsecase) Create(ctx context.Context, realmID, name string, scopes []string) (ServiceAccountCredentials, error) {
	if realmID == "" || name == "" {
		return ServiceAccountCredentials{}, apierrors.InvalidArgument("realm_id and name are required")
	}
	clientID := uuid.NewString()
	rawSecret, secretHash, err := newOpaqueToken()
	if err != nil {
		return ServiceAccountCredentials{}, apierrors.InternalError("failed to generate client secret")
	}

	// The SERVICE account gives the identity a place in the authz graph; the
	// synthetic email satisfies the profile's NOT NULL without colliding with a
	// human login (client_id is unique).
	acc := domain.NewAccount(realmID, clientID+"@svc.aegis.local", name,
		domain.WithAccountType(domain.AccountTypeService),
		domain.WithAccountStatus(domain.AccountStatusEnabled),
	)

	var out domain.ServiceAccount
	err = uc.tx.Exec(ctx, func(ctx context.Context) error {
		created, err := uc.accounts.Create(ctx, acc)
		if err != nil {
			return err
		}
		sa := domain.NewServiceAccount(realmID, name, clientID,
			domain.WithServiceAccountID(created.ID()),
			domain.WithServiceAccountScopes(scopes),
		)
		out, err = uc.repo.Create(ctx, sa, secretHash)
		return err
	})
	if err != nil {
		return ServiceAccountCredentials{}, err
	}
	return ServiceAccountCredentials{ServiceAccount: out, ClientID: clientID, ClientSecret: rawSecret}, nil
}

func (uc *serviceAccountUsecase) Delete(ctx context.Context, accountID string) error {
	if accountID == "" {
		return apierrors.InvalidArgument("account_id is required")
	}
	return uc.tx.Exec(ctx, func(ctx context.Context) error {
		if err := uc.repo.Delete(ctx, accountID); err != nil {
			return err
		}
		_, err := uc.accounts.Patch(ctx,
			repository.PatchSearchOpts(byID(accountID)),
			repository.WithPatchFields(map[string]any{fields.Status: string(domain.AccountStatusDisabled)}),
		)
		return err
	})
}

// IssueToken authenticates the client credentials and mints an access token
// subject'd to the service account, stamping last_used_at.
func (uc *serviceAccountUsecase) IssueToken(ctx context.Context, realmID, issuer, clientID, clientSecret string) (TokenResponse, error) {
	sa, secretHash, err := uc.repo.GetByClientID(ctx, realmID, clientID)
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return TokenResponse{}, apierrors.Unauthenticated("invalid client credentials")
		}
		return TokenResponse{}, err
	}
	if hashToken(clientSecret) != secretHash {
		return TokenResponse{}, apierrors.Unauthenticated("invalid client credentials")
	}

	access, expiresIn, err := uc.tokens.MintAccessToken(ctx, realmID, AccessTokenInput{
		Issuer:   issuer,
		Subject:  sa.ID(),
		Audience: clientID,
		ClientID: clientID,
		Scopes:   sa.Scopes(),
		TTL:      serviceAccountTokenTTL,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	if err := uc.repo.TouchLastUsed(ctx, sa.ID(), uc.now()); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: expiresIn}, nil
}
