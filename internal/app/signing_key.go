package app

import (
	"context"
	"crypto/rsa"
	"encoding/json"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/google/uuid"

	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

const signingAlgRS256 = "RS256"

// SigningKeyRepository persists per-realm signing keys via the kit
// generics. Callers select by realm + status through search options.
type SigningKeyRepository interface {
	repository.Creator[domain.SigningKey]
	repository.Getter[domain.SigningKey]
	repository.Lister[domain.SigningKey]
}

// KeyCipher seals/opens private-key material for storage at rest.
type KeyCipher interface {
	Seal(plaintext []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

// Signer is the active key material used to sign a realm's tokens.
type Signer struct {
	Kid string
	Key *rsa.PrivateKey
}

// SigningKeyService is the realm keystore: signer for minting, public key by kid for verification, JWKS for discovery.
type SigningKeyService interface {
	ActiveSigner(ctx context.Context, realmID string) (Signer, error)
	JWKS(ctx context.Context, realmID string) (json.RawMessage, error)
	VerificationKey(ctx context.Context, realmID, kid string) (*rsa.PublicKey, error)
}

type signingKeyService struct {
	keys   SigningKeyRepository
	cipher KeyCipher
}

func NewSigningKeyService(keys SigningKeyRepository, cipher KeyCipher) SigningKeyService {
	return &signingKeyService{keys: keys, cipher: cipher}
}

func (s *signingKeyService) ActiveSigner(ctx context.Context, realmID string) (Signer, error) {
	key, err := s.keys.Get(ctx, byRealmStatus(realmID, domain.SigningKeyStatusActive))
	if err != nil {
		if !apierrors.Is(err, apierrors.CodeNotFound) {
			return Signer{}, err
		}
		if key, err = s.generate(ctx, realmID); err != nil {
			return Signer{}, err
		}
	}
	der, err := s.cipher.Open(key.SealedPrivateKey())
	if err != nil {
		return Signer{}, apierrors.InternalError("failed to open signing key")
	}
	rsaKey, err := cryptox.ParsePrivateKey(der)
	if err != nil {
		return Signer{}, apierrors.InternalError("failed to parse signing key")
	}
	return Signer{Kid: key.Kid(), Key: rsaKey}, nil
}

func (s *signingKeyService) JWKS(ctx context.Context, realmID string) (json.RawMessage, error) {
	resp, err := s.keys.List(ctx, byRealmPublishable(realmID))
	if err != nil {
		return nil, err
	}
	keys := resp.Results()
	if len(keys) == 0 {
		key, err := s.generate(ctx, realmID)
		if err != nil {
			return nil, err
		}
		keys = []domain.SigningKey{key}
	}
	members := make([]json.RawMessage, 0, len(keys))
	for _, k := range keys {
		members = append(members, k.PublicJWK())
	}
	return cryptox.JWKS(members)
}

func (s *signingKeyService) VerificationKey(ctx context.Context, realmID, kid string) (*rsa.PublicKey, error) {
	resp, err := s.keys.List(ctx, byRealmPublishable(realmID))
	if err != nil {
		return nil, err
	}
	for _, k := range resp.Results() {
		if k.Kid() != kid {
			continue
		}
		pub, err := cryptox.ParsePublicJWK(k.PublicJWK())
		if err != nil {
			return nil, apierrors.InternalError("failed to parse verification key")
		}
		return pub, nil
	}
	return nil, apierrors.NotFound("signing key", kid)
}

// generate mints a new ACTIVE RSA key for the realm, sealing the private half before persisting.
func (s *signingKeyService) generate(ctx context.Context, realmID string) (domain.SigningKey, error) {
	key, err := cryptox.GenerateRSAKey()
	if err != nil {
		return nil, apierrors.InternalError("failed to generate signing key")
	}
	der, err := cryptox.MarshalPrivateKey(key)
	if err != nil {
		return nil, apierrors.InternalError("failed to encode signing key")
	}
	sealed, err := s.cipher.Seal(der)
	if err != nil {
		return nil, apierrors.InternalError("failed to seal signing key")
	}
	kid := uuid.NewString()
	jwk, err := cryptox.PublicJWK(&key.PublicKey, kid)
	if err != nil {
		return nil, apierrors.InternalError("failed to encode public key")
	}
	return s.keys.Create(ctx, domain.NewSigningKey(realmID, kid, signingAlgRS256, jwk, sealed))
}

// byRealmStatus selects a realm's key in a specific status.
func byRealmStatus(realmID string, status domain.SigningKeyStatus) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Status, string(status)),
	)
}

// byRealmPublishable selects a realm's keys whose public half belongs in
// JWKS (ACTIVE + GRACE).
func byRealmPublishable(realmID string) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpIn, fields.Status, []string{
			string(domain.SigningKeyStatusActive),
			string(domain.SigningKeyStatusGrace),
		}),
	)
}
