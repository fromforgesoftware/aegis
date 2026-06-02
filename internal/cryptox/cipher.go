// Package cryptox holds Aegis's signing-key cryptography: envelope
// encryption of private keys at rest, plus RSA/JWK helpers. Kept in one
// leaf package so the app/db layers depend on small, testable primitives.
package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const kekEnvVar = "AEGIS_KEY_ENCRYPTION_KEY"

// Cipher envelope-encrypts secrets: a fresh random data key (DEK) encrypts
// each payload, and the master key (KEK) wraps that DEK. Rotating the KEK
// then only requires re-wrapping DEKs, not re-encrypting payloads.
type Cipher struct {
	kek []byte // 32 bytes → AES-256
}

func NewCipher(kek []byte) (*Cipher, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("key-encryption key must be 32 bytes, got %d", len(kek))
	}
	return &Cipher{kek: kek}, nil
}

// NewCipherFromEnv reads the base64-encoded 32-byte KEK from
// AEGIS_KEY_ENCRYPTION_KEY, failing fast if absent or malformed.
func NewCipherFromEnv() (*Cipher, error) {
	raw := os.Getenv(kekEnvVar)
	if raw == "" {
		return nil, fmt.Errorf("%s is required", kekEnvVar)
	}
	kek, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64: %w", kekEnvVar, err)
	}
	return NewCipher(kek)
}

type envelope struct {
	DEKNonce []byte `json:"dekNonce"`
	DEK      []byte `json:"dek"` // KEK-wrapped data key
	Nonce    []byte `json:"nonce"`
	Payload  []byte `json:"payload"` // DEK-encrypted secret
}

// Seal envelope-encrypts plaintext, returning a self-contained blob safe to
// store at rest.
func (c *Cipher) Seal(plaintext []byte) ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, err
	}
	nonce, payload, err := aesSeal(dek, plaintext)
	if err != nil {
		return nil, err
	}
	dekNonce, wrappedDEK, err := aesSeal(c.kek, dek)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{DEKNonce: dekNonce, DEK: wrappedDEK, Nonce: nonce, Payload: payload})
}

// Open reverses Seal.
func (c *Cipher) Open(sealed []byte) ([]byte, error) {
	var e envelope
	if err := json.Unmarshal(sealed, &e); err != nil {
		return nil, err
	}
	dek, err := aesOpen(c.kek, e.DEKNonce, e.DEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap data key: %w", err)
	}
	return aesOpen(dek, e.Nonce, e.Payload)
}

func aesSeal(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, nil), nil
}

func aesOpen(key, nonce, ciphertext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
