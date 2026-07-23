package authn

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

const testIssuer = "https://aegis.test/realms/demo"

func newTestVerifier(t *testing.T) (*Verifier, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: key.Public(), KeyID: "k1", Algorithm: "RS256", Use: "sig",
	}}}
	body, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	doer := doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	return NewVerifier(testIssuer, WithHTTPClient(doer)), key
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "k1"
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestVerifyExtractsClaims(t *testing.T) {
	v, key := newTestVerifier(t)
	raw := signToken(t, key, jwt.MapClaims{
		"iss":      testIssuer,
		"sub":      "acct-1",
		"org_id":   "org-1",
		"org_role": "ADMIN",
		"rid":      "realm-uuid",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	c, err := v.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	want := Claims{Subject: "acct-1", OrgID: "org-1", OrgRole: "ADMIN", Issuer: testIssuer, RealmID: "realm-uuid"}
	if c != want {
		t.Fatalf("claims = %+v, want %+v", c, want)
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	v, key := newTestVerifier(t)
	raw := signToken(t, key, jwt.MapClaims{
		"iss": "https://evil.test",
		"sub": "acct-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Verify(context.Background(), raw); err == nil {
		t.Fatal("expected issuer mismatch error")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	v, key := newTestVerifier(t)
	raw := signToken(t, key, jwt.MapClaims{
		"iss": testIssuer,
		"sub": "acct-1",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	if _, err := v.Verify(context.Background(), raw); err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestVerifyRejectsUnknownKey(t *testing.T) {
	v, _ := newTestVerifier(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw := signToken(t, other, jwt.MapClaims{
		"iss": testIssuer,
		"sub": "acct-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Verify(context.Background(), raw); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestAuthenticatorInjectsClaimsAndRawToken(t *testing.T) {
	v, key := newTestVerifier(t)
	raw := signToken(t, key, jwt.MapClaims{
		"iss":    testIssuer,
		"sub":    "acct-1",
		"org_id": "org-1",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	a := NewAuthenticator(v)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+raw)
	if err := a.Authenticate(r); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	c, ok := ClaimsFromCtx(r.Context())
	if !ok || c.Subject != "acct-1" {
		t.Fatalf("claims not injected: %+v ok=%v", c, ok)
	}
	if got, ok := RawTokenFromCtx(r.Context()); !ok || got != raw {
		t.Fatal("raw token not injected")
	}
	if OwnerFromCtx(r.Context()) != "org-1" || OrgIDFromCtx(r.Context()) != "org-1" {
		t.Fatal("owner/org helpers wrong")
	}

	missing := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := a.Authenticate(missing); err == nil || !strings.Contains(err.Error(), "missing bearer token") {
		t.Fatalf("expected missing-token error, got %v", err)
	}
}
