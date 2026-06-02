package cryptox

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"

	jose "github.com/go-jose/go-jose/v4"
)

const rsaKeyBits = 2048

// GenerateRSAKey makes a fresh RS256 signing key.
func GenerateRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, rsaKeyBits)
}

// MarshalPrivateKey serializes a private key to PKCS#8 DER (the bytes that
// get envelope-sealed before storage).
func MarshalPrivateKey(key *rsa.PrivateKey) ([]byte, error) {
	return x509.MarshalPKCS8PrivateKey(key)
}

// ParsePrivateKey reverses MarshalPrivateKey.
func ParsePrivateKey(der []byte) (*rsa.PrivateKey, error) {
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("decoded key is not RSA")
	}
	return rk, nil
}

// PublicJWK renders an RSA public key as a JWK (RFC 7517) for JWKS.
func PublicJWK(pub *rsa.PublicKey, kid string) (json.RawMessage, error) {
	jwk := jose.JSONWebKey{Key: pub, KeyID: kid, Algorithm: "RS256", Use: "sig"}
	return jwk.MarshalJSON()
}

// ParsePublicJWK reverses PublicJWK, recovering the RSA public key used to
// verify a token signed under that JWK's kid.
func ParsePublicJWK(raw json.RawMessage) (*rsa.PublicKey, error) {
	var jwk jose.JSONWebKey
	if err := jwk.UnmarshalJSON(raw); err != nil {
		return nil, err
	}
	pub, ok := jwk.Key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("JWK is not an RSA public key")
	}
	return pub, nil
}

// JWKS assembles a JWK Set document ({"keys":[...]}) from raw JWK members.
func JWKS(members []json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(struct {
		Keys []json.RawMessage `json:"keys"`
	}{Keys: members})
}
