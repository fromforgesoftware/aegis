package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// HTTPDoer is the request surface every connector needs from net/http.
// Tests pass a stub; production passes *http.Client.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// OIDCConnector implements Connector for ExternalIDPKindOIDCCustom by
// fetching the upstream discovery document + JWKS and verifying the user's
// ID token against the keys advertised there.
type OIDCConnector struct {
	http HTTPDoer
	now  func() time.Time
}

func NewOIDCConnector(http HTTPDoer) *OIDCConnector {
	if http == nil {
		http = defaultHTTPClient()
	}
	return &OIDCConnector{http: http, now: time.Now}
}

func defaultHTTPClient() HTTPDoer {
	return &http.Client{Timeout: 5 * time.Second}
}

func (c *OIDCConnector) Kind() domain.ExternalIDPKind { return domain.ExternalIDPKindOIDCCustom }

func (c *OIDCConnector) Verify(ctx context.Context, cfg domain.ExternalIDPConfig, rawToken string) (ExternalUser, error) {
	if cfg.DiscoveryURL() == "" {
		return ExternalUser{}, apierrors.InvalidArgument("custom OIDC IdP missing discovery_url")
	}
	disc, err := c.fetchDiscovery(cfg.DiscoveryURL())
	if err != nil {
		return ExternalUser{}, err
	}
	keys, err := c.fetchJWKS(disc.JWKSURI)
	if err != nil {
		return ExternalUser{}, err
	}

	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(rawToken, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, apierrors.Unauthenticated("token has no kid")
		}
		for _, k := range keys.Keys {
			if k.KeyID == kid {
				return k.Key, nil
			}
		}
		return nil, apierrors.Unauthenticated("no JWKS key matches the token kid")
	}, jwt.WithValidMethods([]string{"RS256", "ES256"})); err != nil {
		return ExternalUser{}, apierrors.Unauthenticated("invalid ID token")
	}

	if iss, _ := claims["iss"].(string); iss != disc.Issuer {
		return ExternalUser{}, apierrors.Unauthenticated("issuer mismatch")
	}
	if cfg.ClientID() != "" && !audienceMatches(claims["aud"], cfg.ClientID()) {
		return ExternalUser{}, apierrors.Unauthenticated("audience mismatch")
	}

	user := ExternalUser{}
	if v, ok := claims["sub"].(string); ok {
		user.ID = v
	}
	if v, ok := claims["email"].(string); ok {
		user.Email = v
	}
	if v, ok := claims["email_verified"].(bool); ok {
		user.EmailVerified = v
	}
	if v, ok := claims["name"].(string); ok {
		user.Name = v
	}
	return user, nil
}

type oidcDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

func (c *OIDCConnector) fetchDiscovery(url string) (oidcDiscovery, error) {
	var d oidcDiscovery
	if err := c.fetchJSON(url, &d); err != nil {
		return oidcDiscovery{}, err
	}
	if d.Issuer == "" || d.JWKSURI == "" {
		return oidcDiscovery{}, apierrors.InvalidArgument("discovery document is missing issuer or jwks_uri")
	}
	return d, nil
}

func (c *OIDCConnector) fetchJWKS(url string) (*jose.JSONWebKeySet, error) {
	var set jose.JSONWebKeySet
	if err := c.fetchJSON(url, &set); err != nil {
		return nil, err
	}
	return &set, nil
}

func (c *OIDCConnector) fetchJSON(url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return apierrors.InternalError("failed to build upstream IdP request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return apierrors.InternalError("failed to reach upstream IdP")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apierrors.InternalError("upstream IdP returned non-2xx")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apierrors.InternalError("failed to read upstream IdP response")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return apierrors.InternalError("upstream IdP response is not valid JSON")
	}
	return nil
}

// audienceMatches accepts the JWT aud claim as either a string or a string
// array (RFC 7519 §4.1.3) and reports whether want is present.
func audienceMatches(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s == want {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == want {
				return true
			}
		}
	}
	return false
}
