package domain

import (
	"encoding/json"
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeSigningKey is the JSON:API type for the signing-key admin
// resource (key-rotation views, later wave).
const ResourceTypeSigningKey resource.Type = "signingKeys"

// SigningKeyStatus is the lifecycle marker of a realm signing key: ACTIVE
// signs new tokens, GRACE only verifies (its public key stays in JWKS
// during rotation), RETIRED is dropped.
type SigningKeyStatus string

const (
	SigningKeyStatusActive  SigningKeyStatus = "ACTIVE"
	SigningKeyStatusGrace   SigningKeyStatus = "GRACE"
	SigningKeyStatusRetired SigningKeyStatus = "RETIRED"
)

func (s SigningKeyStatus) Valid() bool {
	switch s {
	case SigningKeyStatusActive, SigningKeyStatusGrace, SigningKeyStatusRetired:
		return true
	}
	return false
}

// SigningKey is a per-realm RS256 key. It is a resource so the keystore
// repo uses the generic Getter/Lister/Creator; the public half feeds JWKS
// and the (later) admin view, while SealedPrivateKey is envelope-encrypted
// material consumed only by the keystore and never serialized to a DTO.
type SigningKey interface {
	resource.Resource
	RealmID() string
	Kid() string
	Algorithm() string
	PublicJWK() json.RawMessage
	Status() SigningKeyStatus
	SealedPrivateKey() []byte
	NotBefore() time.Time
	NotAfter() *time.Time
}

type signingKey struct {
	resource.Resource

	realmID          string
	kid              string
	algorithm        string
	publicJWK        json.RawMessage
	status           SigningKeyStatus
	sealedPrivateKey []byte
	notBefore        time.Time
	notAfter         *time.Time
}

type SigningKeyOption func(*signingKey)

func WithSigningKeyID(id string) SigningKeyOption {
	return func(k *signingKey) { k.Resource = resource.Update(k.Resource, resource.WithID(id)) }
}
func WithSigningKeyNotAfter(t time.Time) SigningKeyOption {
	return func(k *signingKey) { k.notAfter = &t }
}

// NewSigningKey builds a signing-key aggregate (default status ACTIVE,
// not_before now).
func NewSigningKey(realmID, kid, algorithm string, publicJWK json.RawMessage, sealedPrivateKey []byte, opts ...SigningKeyOption) SigningKey {
	k := &signingKey{
		Resource:         resource.New(resource.WithType(ResourceTypeSigningKey)),
		realmID:          realmID,
		kid:              kid,
		algorithm:        algorithm,
		publicJWK:        publicJWK,
		sealedPrivateKey: sealedPrivateKey,
		status:           SigningKeyStatusActive,
		notBefore:        time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

func (k *signingKey) RealmID() string            { return k.realmID }
func (k *signingKey) Kid() string                { return k.kid }
func (k *signingKey) Algorithm() string          { return k.algorithm }
func (k *signingKey) PublicJWK() json.RawMessage { return k.publicJWK }
func (k *signingKey) Status() SigningKeyStatus   { return k.status }
func (k *signingKey) SealedPrivateKey() []byte   { return k.sealedPrivateKey }
func (k *signingKey) NotBefore() time.Time       { return k.notBefore }
func (k *signingKey) NotAfter() *time.Time       { return k.notAfter }
