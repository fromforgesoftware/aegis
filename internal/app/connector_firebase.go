package app

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/golang-jwt/jwt/v5"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// firebaseCertsURL serves x509 certificates (PEM, keyed by kid) for the
// secure-token signer Firebase uses to sign ID tokens.
const firebaseCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

// firebaseIssuerPrefix + project_id forms a Firebase ID-token issuer.
const firebaseIssuerPrefix = "https://securetoken.google.com/"

// FirebaseConnector verifies Firebase ID tokens. Unlike standard OIDC,
// Firebase doesn't publish JWKS — it ships x509 PEM certs keyed by kid at
// firebaseCertsURL. project_id (from cfg.Config) anchors the issuer + aud
// checks.
type FirebaseConnector struct {
	http HTTPDoer
}

func NewFirebaseConnector(http HTTPDoer) *FirebaseConnector {
	if http == nil {
		http = defaultHTTPClient()
	}
	return &FirebaseConnector{http: http}
}

func (c *FirebaseConnector) Kind() domain.ExternalIDPKind { return domain.ExternalIDPKindFirebase }

func (c *FirebaseConnector) Verify(ctx context.Context, cfg domain.ExternalIDPConfig, rawToken string) (ExternalUser, error) {
	projectID := cfg.Config()["project_id"]
	if projectID == "" {
		return ExternalUser{}, apierrors.InvalidArgument("firebase IdP missing config.project_id")
	}
	keys, err := c.fetchCerts()
	if err != nil {
		return ExternalUser{}, err
	}

	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(rawToken, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, apierrors.Unauthenticated("no Firebase cert matches the token kid")
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"})); err != nil {
		return ExternalUser{}, apierrors.Unauthenticated("invalid Firebase ID token")
	}

	if iss, _ := claims["iss"].(string); iss != firebaseIssuerPrefix+projectID {
		return ExternalUser{}, apierrors.Unauthenticated("issuer mismatch")
	}
	if !audienceMatches(claims["aud"], projectID) {
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

// fetchCerts reads the kid→PEM-cert map and parses each PEM into the RSA
// public key the JWT validator needs.
func (c *FirebaseConnector) fetchCerts() (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequest(http.MethodGet, firebaseCertsURL, nil)
	if err != nil {
		return nil, apierrors.InternalError("failed to build Firebase cert request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, apierrors.InternalError("failed to reach Firebase cert endpoint")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, apierrors.InternalError("Firebase cert endpoint returned non-2xx")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apierrors.InternalError("failed to read Firebase cert response")
	}
	var raw map[string]string
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, apierrors.InternalError("Firebase cert response is not valid JSON")
	}
	out := make(map[string]*rsa.PublicKey, len(raw))
	for kid, pemStr := range raw {
		block, _ := pem.Decode([]byte(pemStr))
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			continue
		}
		out[kid] = pub
	}
	if len(out) == 0 {
		return nil, apierrors.InternalError("Firebase cert response contained no usable RSA keys")
	}
	return out, nil
}
