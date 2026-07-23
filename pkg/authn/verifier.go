// Package authn authenticates callers of a consumer service using aegis
// realm-issued RS256 access tokens, verified offline against the realm's
// public JWKS — no call to the aegis service on the request hot path.
//
// This is the SDK for services that trust an aegis realm as their issuer
// (an edge API, a gateway). It is distinct from pkg/client/middleware, which
// resolves external-IdP tokens to accounts by calling aegis per request.
package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

// HTTPDoer is the request surface (so tests can stub the JWKS fetch).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Claims is the identity extracted from a verified access token.
type Claims struct {
	Subject string
	OrgID   string
	OrgRole string
	Issuer  string
	// RealmID is the aegis realm's UUID (the `rid` claim). Distinct from Issuer,
	// which is the realm's URL/name. Used to stamp audit events (talos realm_id is
	// a UUID). Empty for tokens minted before aegis added the claim.
	RealmID string
}

// Verifier validates bearer tokens against one aegis realm, caching the JWKS.
type Verifier struct {
	issuer  string
	jwksURL string
	http    HTTPDoer
	ttl     time.Duration

	mu        sync.Mutex
	keys      *jose.JSONWebKeySet
	fetchedAt time.Time
	now       func() time.Time
}

type Option func(*Verifier)

func WithHTTPClient(h HTTPDoer) Option      { return func(v *Verifier) { v.http = h } }
func WithJWKSURL(u string) Option           { return func(v *Verifier) { v.jwksURL = u } }
func WithClock(now func() time.Time) Option { return func(v *Verifier) { v.now = now } }

// NewVerifier targets the realm whose issuer is `issuer`
// (e.g. https://aegis.example/realms/trading-bot). The JWKS defaults to
// issuer + /.well-known/jwks.json; WithJWKSURL decouples the key fetch from
// the issuer host (e.g. in-cluster DNS while tokens carry the public host).
func NewVerifier(issuer string, opts ...Option) *Verifier {
	v := &Verifier{
		issuer: strings.TrimRight(issuer, "/"),
		http:   &http.Client{Timeout: 5 * time.Second},
		ttl:    time.Hour,
		now:    time.Now,
	}
	v.jwksURL = v.issuer + "/.well-known/jwks.json"
	for _, o := range opts {
		o(v)
	}
	return v
}

// Verify checks the token's RS256 signature against the realm JWKS, its issuer,
// and expiry, returning the caller's claims.
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	keys, err := v.jwks(ctx)
	if err != nil {
		return Claims{}, err
	}
	mc := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(raw, mc, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		for i := range keys.Keys {
			if keys.Keys[i].KeyID == kid {
				return keys.Keys[i].Key, nil
			}
		}
		return nil, fmt.Errorf("no JWKS key matches kid %q", kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithTimeFunc(v.now))
	if err != nil {
		return Claims{}, fmt.Errorf("invalid token: %w", err)
	}
	if iss, _ := mc["iss"].(string); iss != v.issuer {
		return Claims{}, fmt.Errorf("issuer mismatch: %q != %q", iss, v.issuer)
	}
	c := Claims{Issuer: v.issuer}
	c.Subject, _ = mc["sub"].(string)
	c.OrgID, _ = mc["org_id"].(string)
	c.OrgRole, _ = mc["org_role"].(string)
	c.RealmID, _ = mc["rid"].(string)
	return c, nil
}

func (v *Verifier) jwks(ctx context.Context) (*jose.JSONWebKeySet, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys != nil && v.now().Sub(v.fetchedAt) < v.ttl {
		return v.keys, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch JWKS: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var set jose.JSONWebKeySet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}
	v.keys = &set
	v.fetchedAt = v.now()
	return v.keys, nil
}
