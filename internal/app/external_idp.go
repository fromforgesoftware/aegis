package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ExternalIDPConfigRepository persists per-realm IdP configs via kit generics.
type ExternalIDPConfigRepository interface {
	repository.Creator[domain.ExternalIDPConfig]
	repository.Getter[domain.ExternalIDPConfig]
	repository.Lister[domain.ExternalIDPConfig]
	repository.Patcher[domain.ExternalIDPConfig]
	repository.Deleter
}

// ExternalIDPSecret carries a raw upstream client_secret submitted at create
// time; the usecase envelope-encrypts it via KeyCipher before persistence.
type ExternalIDPSecret string

// ExternalIDPConfigUsecase is the admin surface for IdP configurations.
type ExternalIDPConfigUsecase interface {
	repository.Creator[domain.ExternalIDPConfig]
	repository.Getter[domain.ExternalIDPConfig]
	repository.Lister[domain.ExternalIDPConfig]
	repository.Patcher[domain.ExternalIDPConfig]
	repository.Deleter
	// CreateWithSecret accepts the raw upstream secret, seals it, and persists.
	CreateWithSecret(ctx context.Context, c domain.ExternalIDPConfig, secret string) (domain.ExternalIDPConfig, error)
}

type externalIDPConfigUsecase struct {
	usecase.Getter[domain.ExternalIDPConfig]
	usecase.Lister[domain.ExternalIDPConfig]
	repository.Patcher[domain.ExternalIDPConfig]
	repository.Deleter

	repo   ExternalIDPConfigRepository
	cipher KeyCipher
}

func NewExternalIDPConfigUsecase(repo ExternalIDPConfigRepository, cipher KeyCipher) ExternalIDPConfigUsecase {
	return &externalIDPConfigUsecase{
		Getter:  usecase.NewGetter(repo, domain.ResourceTypeExternalIDP),
		Lister:  usecase.NewLister(repo),
		Patcher: repo,
		Deleter: usecase.NewDeleter(repo),
		repo:    repo,
		cipher:  cipher,
	}
}

func (uc *externalIDPConfigUsecase) Create(ctx context.Context, c domain.ExternalIDPConfig) (domain.ExternalIDPConfig, error) {
	if err := validateExternalIDPConfig(c); err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, c)
}

func (uc *externalIDPConfigUsecase) CreateWithSecret(ctx context.Context, c domain.ExternalIDPConfig, secret string) (domain.ExternalIDPConfig, error) {
	if err := validateExternalIDPConfig(c); err != nil {
		return nil, err
	}
	if secret == "" {
		return uc.repo.Create(ctx, c)
	}
	sealed, err := uc.cipher.Seal([]byte(secret))
	if err != nil {
		return nil, apierrors.InternalError("failed to seal upstream client secret")
	}
	return uc.repo.Create(ctx, withExternalIDPSecret(c, sealed))
}

func validateExternalIDPConfig(c domain.ExternalIDPConfig) error {
	if c.RealmID() == "" {
		return apierrors.InvalidArgument("realm_id is required")
	}
	if !c.Kind().Valid() {
		return apierrors.InvalidArgument("invalid external IdP kind")
	}
	if c.Name() == "" {
		return apierrors.InvalidArgument("name is required")
	}
	return nil
}

// withExternalIDPSecret rebuilds the create input with the sealed secret;
// id/timestamps stay server-assigned.
func withExternalIDPSecret(c domain.ExternalIDPConfig, sealed []byte) domain.ExternalIDPConfig {
	return domain.NewExternalIDPConfig(c.RealmID(), c.Kind(), c.Name(),
		domain.WithExternalIDPEnabled(c.Enabled()),
		domain.WithExternalIDPClientID(c.ClientID()),
		domain.WithExternalIDPClientSecretEncrypted(sealed),
		domain.WithExternalIDPDiscoveryURL(c.DiscoveryURL()),
		domain.WithExternalIDPIssuer(c.Issuer()),
		domain.WithExternalIDPScopes(c.Scopes()),
		domain.WithExternalIDPConfig(c.Config()),
	)
}
