package app

import (
	"context"
	"strings"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessTokenInput is the data for an RFC 9068 JWT access token.
type AccessTokenInput struct {
	Issuer   string
	Subject  string // account id
	Audience string
	ClientID string
	Scopes   []string
	OrgID    string // active organization (omitted when empty)
	OrgRole  string // caller's top role on the active org
	TTL      time.Duration
}

// IDTokenInput is the data for an OIDC ID token.
type IDTokenInput struct {
	Issuer        string
	Subject       string
	Audience      string // client_id
	Nonce         string
	Email         string
	EmailVerified bool
	Name          string
	TTL           time.Duration
}

// AccessClaims are the verified claims of an access-token JWT, surfaced by
// token introspection.
type AccessClaims struct {
	Subject   string
	ClientID  string
	Scope     string
	Audience  string
	Issuer    string
	OrgID     string
	OrgRole   string
	IssuedAt  int64
	ExpiresAt int64
}

// TokenIssuer mints and verifies RS256 JWTs signed by a realm's signing keys
// (kid in the header so verifiers select the right JWKS entry).
type TokenIssuer interface {
	MintAccessToken(ctx context.Context, realmID string, in AccessTokenInput) (token string, expiresIn int64, err error)
	MintIDToken(ctx context.Context, realmID string, in IDTokenInput) (string, error)
	// VerifyAccessToken validates an access-token JWT's signature (by kid) and
	// standard time claims, returning its claims. An invalid or expired token
	// yields an error.
	VerifyAccessToken(ctx context.Context, realmID, token string) (AccessClaims, error)
}

type tokenIssuer struct {
	keys SigningKeyService
}

func NewTokenIssuer(keys SigningKeyService) TokenIssuer {
	return &tokenIssuer{keys: keys}
}

func (i *tokenIssuer) MintAccessToken(ctx context.Context, realmID string, in AccessTokenInput) (string, int64, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":       in.Issuer,
		"sub":       in.Subject,
		"aud":       in.Audience,
		"iat":       now.Unix(),
		"exp":       now.Add(in.TTL).Unix(),
		"jti":       uuid.NewString(),
		"client_id": in.ClientID,
		"scope":     strings.Join(in.Scopes, " "),
	}
	if in.OrgID != "" {
		claims["org_id"] = in.OrgID
		if in.OrgRole != "" {
			claims["org_role"] = in.OrgRole
		}
	}
	tok, err := i.sign(ctx, realmID, claims, "at+jwt")
	if err != nil {
		return "", 0, err
	}
	return tok, int64(in.TTL.Seconds()), nil
}

func (i *tokenIssuer) MintIDToken(ctx context.Context, realmID string, in IDTokenInput) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": in.Issuer,
		"sub": in.Subject,
		"aud": in.Audience,
		"iat": now.Unix(),
		"exp": now.Add(in.TTL).Unix(),
	}
	if in.Nonce != "" {
		claims["nonce"] = in.Nonce
	}
	if in.Email != "" {
		claims["email"] = in.Email
		claims["email_verified"] = in.EmailVerified
	}
	if in.Name != "" {
		claims["name"] = in.Name
	}
	return i.sign(ctx, realmID, claims, "JWT")
}

func (i *tokenIssuer) VerifyAccessToken(ctx context.Context, realmID, token string) (AccessClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, apierrors.Unauthenticated("unexpected signing method")
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, apierrors.Unauthenticated("token has no kid")
		}
		return i.keys.VerificationKey(ctx, realmID, kid)
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return AccessClaims{}, apierrors.Unauthenticated("invalid token")
	}

	out := AccessClaims{}
	if v, ok := claims["sub"].(string); ok {
		out.Subject = v
	}
	if v, ok := claims["client_id"].(string); ok {
		out.ClientID = v
	}
	if v, ok := claims["scope"].(string); ok {
		out.Scope = v
	}
	if v, ok := claims["aud"].(string); ok {
		out.Audience = v
	}
	if v, ok := claims["iss"].(string); ok {
		out.Issuer = v
	}
	if v, ok := claims["org_id"].(string); ok {
		out.OrgID = v
	}
	if v, ok := claims["org_role"].(string); ok {
		out.OrgRole = v
	}
	if v, ok := claims["iat"].(float64); ok {
		out.IssuedAt = int64(v)
	}
	if v, ok := claims["exp"].(float64); ok {
		out.ExpiresAt = int64(v)
	}
	return out, nil
}

func (i *tokenIssuer) sign(ctx context.Context, realmID string, claims jwt.MapClaims, typ string) (string, error) {
	signer, err := i.keys.ActiveSigner(ctx, realmID)
	if err != nil {
		return "", err
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = signer.Kid
	tok.Header["typ"] = typ
	s, err := tok.SignedString(signer.Key)
	if err != nil {
		return "", apierrors.InternalError("failed to sign token")
	}
	return s, nil
}
