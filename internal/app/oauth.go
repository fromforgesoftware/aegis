package app

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

const (
	sessionTTL      = 12 * time.Hour
	authCodeTTL     = 1 * time.Minute
	accessTokenTTL  = 1 * time.Hour
	idTokenTTL      = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

// SessionRepository persists browser login sessions. Revoke stamps a
// revoked_at on the session — the kit's SessionActive helper then rejects it,
// which kills any refresh-token chain anchored to the session.
type SessionRepository interface {
	repository.Creator[domain.Session]
	repository.Getter[domain.Session]
	Revoke(ctx context.Context, sessionID string, now time.Time) error
}

// ActiveOrgResolver resolves the active organization an access token should
// carry for an account (its sole membership, with the top role). Implemented
// by the organization usecase; empty when the account has none or many orgs.
type ActiveOrgResolver interface {
	ActiveOrg(ctx context.Context, accountID string) (orgID, orgRole string, err error)
}

// AuthorizationCodeRepository persists single-use authorization codes.
type AuthorizationCodeRepository interface {
	Create(ctx context.Context, code domain.AuthorizationCode) error
	// Consume atomically marks a valid code consumed and returns it.
	Consume(ctx context.Context, code string, now time.Time) (domain.AuthorizationCode, error)
}

// RefreshTokenRepository persists rotated refresh tokens (hashed at rest) and
// the reuse-detection machinery.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token domain.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	// MarkUsed atomically claims an unused token, reporting whether this call
	// claimed it (false ⇒ a concurrent reuse already rotated it).
	MarkUsed(ctx context.Context, id string, now time.Time) (bool, error)
}

// AuthorizeInput is a validated /authorize request for an authenticated user.
type AuthorizeInput struct {
	RealmID             string
	ClientID            string
	AccountID           string
	SessionID           string
	RedirectURI         string
	Scopes              []string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// AuthorizeResult is what the controller redirects with.
type AuthorizeResult struct {
	Code        string
	RedirectURI string
	State       string
}

// CodeExchangeInput is a /token authorization_code grant request.
type CodeExchangeInput struct {
	RealmID      string
	Issuer       string
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

// RefreshInput is a /token refresh_token grant request.
type RefreshInput struct {
	RealmID      string
	Issuer       string
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// IntrospectInput is a client-authenticated RFC 7662 introspection request.
type IntrospectInput struct {
	RealmID       string
	ClientID      string
	ClientSecret  string
	Token         string
	TokenTypeHint string
}

// IntrospectionResult is the RFC 7662 response: Active plus the token's claims
// when it is valid.
type IntrospectionResult struct {
	Active    bool
	Scope     string
	ClientID  string
	Subject   string
	TokenType string
	Exp       int64
	Iat       int64
	Audience  string
	Issuer    string
	OrgID     string
	OrgRole   string
}

// RevokeInput is a client-authenticated RFC 7009 revocation request.
type RevokeInput struct {
	RealmID       string
	ClientID      string
	ClientSecret  string
	Token         string
	TokenTypeHint string
}

// ClientCredentialsInput is a /token client_credentials grant request
// (RFC 6749 §4.4) — a confidential client acting as its own principal.
type ClientCredentialsInput struct {
	RealmID      string
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// TokenResponse is the RFC 6749 §5.1 token endpoint response.
type TokenResponse struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int64
	IDToken      string
	RefreshToken string
	Scope        string
}

// OAuthUsecase drives the OAuth2/OIDC grants and the login session that
// /authorize authenticates against.
type OAuthUsecase interface {
	StartSession(ctx context.Context, realmID, accountID string) (sessionID string, err error)
	ResolveSession(ctx context.Context, sessionID string) (domain.Session, error)
	EndSession(ctx context.Context, sessionID string) error
	Authorize(ctx context.Context, in AuthorizeInput) (AuthorizeResult, error)
	ExchangeCode(ctx context.Context, in CodeExchangeInput) (TokenResponse, error)
	ClientCredentials(ctx context.Context, in ClientCredentialsInput) (TokenResponse, error)
	Refresh(ctx context.Context, in RefreshInput) (TokenResponse, error)
	Introspect(ctx context.Context, in IntrospectInput) (IntrospectionResult, error)
	Revoke(ctx context.Context, in RevokeInput) error
}

type oauthUsecase struct {
	clients   ClientRepository
	accounts  AccountRepository
	authCodes AuthorizationCodeRepository
	refresh   RefreshTokenRepository
	sessions  SessionRepository
	tokens    TokenIssuer
	orgs      ActiveOrgResolver
}

func NewOAuthUsecase(
	clients ClientRepository,
	accounts AccountRepository,
	authCodes AuthorizationCodeRepository,
	refresh RefreshTokenRepository,
	sessions SessionRepository,
	tokens TokenIssuer,
	orgs ActiveOrgResolver,
) OAuthUsecase {
	return &oauthUsecase{clients: clients, accounts: accounts, authCodes: authCodes, refresh: refresh, sessions: sessions, tokens: tokens, orgs: orgs}
}

func (uc *oauthUsecase) StartSession(ctx context.Context, realmID, accountID string) (string, error) {
	s, err := uc.sessions.Create(ctx, domain.NewSession(realmID, accountID, time.Now().UTC().Add(sessionTTL)))
	if err != nil {
		return "", err
	}
	return s.ID(), nil
}

func (uc *oauthUsecase) ResolveSession(ctx context.Context, sessionID string) (domain.Session, error) {
	s, err := uc.sessions.Get(ctx, byID(sessionID))
	if err != nil {
		return nil, err
	}
	if !domain.SessionActive(s, time.Now().UTC()) {
		return nil, apierrors.Unauthenticated("session expired")
	}
	return s, nil
}

// EndSession revokes a browser login session (RP-initiated logout). Unknown or
// already-revoked sessions are treated as success — logout is idempotent.
func (uc *oauthUsecase) EndSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if err := uc.sessions.Revoke(ctx, sessionID, time.Now().UTC()); err != nil && !apierrors.Is(err, apierrors.CodeNotFound) {
		return err
	}
	return nil
}

func (uc *oauthUsecase) Authorize(ctx context.Context, in AuthorizeInput) (AuthorizeResult, error) {
	client, err := uc.clients.Get(ctx, byRealmClientID(in.RealmID, in.ClientID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return AuthorizeResult{}, apierrors.InvalidArgument("unknown client")
		}
		return AuthorizeResult{}, err
	}
	if !slices.Contains(client.RedirectURIs(), in.RedirectURI) {
		return AuthorizeResult{}, apierrors.InvalidArgument("redirect_uri not registered for client")
	}
	if in.CodeChallengeMethod != "" && in.CodeChallengeMethod != "S256" {
		return AuthorizeResult{}, apierrors.InvalidArgument("only S256 code_challenge_method is supported")
	}
	if client.PKCERequired() && in.CodeChallenge == "" {
		return AuthorizeResult{}, apierrors.InvalidArgument("code_challenge is required (PKCE)")
	}

	code, _, err := newOpaqueToken()
	if err != nil {
		return AuthorizeResult{}, apierrors.InternalError("failed to generate authorization code")
	}
	if err := uc.authCodes.Create(ctx, domain.AuthorizationCode{
		Code:          code,
		RealmID:       in.RealmID,
		ClientID:      in.ClientID,
		AccountID:     in.AccountID,
		SessionID:     in.SessionID,
		RedirectURI:   in.RedirectURI,
		Scopes:        in.Scopes,
		PKCEChallenge: in.CodeChallenge,
		Nonce:         in.Nonce,
		ExpiresAt:     time.Now().UTC().Add(authCodeTTL),
	}); err != nil {
		return AuthorizeResult{}, err
	}
	return AuthorizeResult{Code: code, RedirectURI: in.RedirectURI, State: in.State}, nil
}

func (uc *oauthUsecase) ExchangeCode(ctx context.Context, in CodeExchangeInput) (TokenResponse, error) {
	authCode, err := uc.authCodes.Consume(ctx, in.Code, time.Now().UTC())
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return TokenResponse{}, apierrors.InvalidArgument("invalid or expired authorization code")
		}
		return TokenResponse{}, err
	}
	if authCode.ClientID != in.ClientID {
		return TokenResponse{}, apierrors.InvalidArgument("authorization code was issued to a different client")
	}
	if authCode.RedirectURI != in.RedirectURI {
		return TokenResponse{}, apierrors.InvalidArgument("redirect_uri mismatch")
	}

	client, err := uc.clients.Get(ctx, byRealmClientID(in.RealmID, in.ClientID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return TokenResponse{}, apierrors.Unauthenticated("unknown client")
		}
		return TokenResponse{}, err
	}
	if err := authenticateClient(client, in.ClientSecret); err != nil {
		return TokenResponse{}, err
	}
	if authCode.PKCEChallenge != "" {
		if in.CodeVerifier == "" || !cryptox.VerifyPKCES256(in.CodeVerifier, authCode.PKCEChallenge) {
			return TokenResponse{}, apierrors.InvalidArgument("PKCE verification failed")
		}
	}

	resp, err := uc.issueTokens(ctx, in.RealmID, in.Issuer, in.ClientID, authCode.AccountID, authCode.Scopes, authCode.Nonce)
	if err != nil {
		return TokenResponse{}, err
	}

	if slices.Contains(client.GrantTypes(), "refresh_token") && authCode.SessionID != "" {
		raw, err := uc.mintRefreshToken(ctx, authCode.SessionID, in.ClientID, authCode.Scopes, "")
		if err != nil {
			return TokenResponse{}, err
		}
		resp.RefreshToken = raw
	}
	return resp, nil
}

// Refresh exchanges a refresh token for a new access token and a rotated refresh token (RFC 6749 §6); reuse of an already-rotated token revokes the session.
func (uc *oauthUsecase) Refresh(ctx context.Context, in RefreshInput) (TokenResponse, error) {
	client, err := uc.clients.Get(ctx, byRealmClientID(in.RealmID, in.ClientID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return TokenResponse{}, apierrors.Unauthenticated("unknown client")
		}
		return TokenResponse{}, err
	}
	if err := authenticateClient(client, in.ClientSecret); err != nil {
		return TokenResponse{}, err
	}

	now := time.Now().UTC()
	rt, err := uc.refresh.GetByHash(ctx, hashToken(in.RefreshToken))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return TokenResponse{}, apierrors.InvalidArgument("invalid refresh token")
		}
		return TokenResponse{}, err
	}
	if rt.ClientID != in.ClientID {
		return TokenResponse{}, apierrors.InvalidArgument("refresh token was issued to a different client")
	}
	if rt.UsedAt != nil {
		if rerr := uc.sessions.Revoke(ctx, rt.SessionID, now); rerr != nil {
			return TokenResponse{}, rerr
		}
		return TokenResponse{}, apierrors.InvalidArgument("refresh token reuse detected")
	}
	if !now.Before(rt.ExpiresAt) {
		return TokenResponse{}, apierrors.InvalidArgument("refresh token expired")
	}

	claimed, err := uc.refresh.MarkUsed(ctx, rt.ID, now)
	if err != nil {
		return TokenResponse{}, err
	}
	if !claimed {
		return TokenResponse{}, apierrors.InvalidArgument("refresh token reuse detected")
	}

	sess, err := uc.sessions.Get(ctx, byID(rt.SessionID))
	if err != nil {
		return TokenResponse{}, err
	}
	if !domain.SessionActive(sess, now) || sess.RealmID() != in.RealmID {
		return TokenResponse{}, apierrors.InvalidArgument("session is no longer active")
	}

	resp, err := uc.issueTokens(ctx, in.RealmID, in.Issuer, in.ClientID, sess.AccountID(), rt.Scopes, "")
	if err != nil {
		return TokenResponse{}, err
	}
	raw, err := uc.mintRefreshToken(ctx, rt.SessionID, in.ClientID, rt.Scopes, rt.ID)
	if err != nil {
		return TokenResponse{}, err
	}
	resp.RefreshToken = raw
	return resp, nil
}

// mintRefreshToken returns the opaque token handed to the client; only its hash is persisted.
func (uc *oauthUsecase) mintRefreshToken(ctx context.Context, sessionID, clientID string, scopes []string, rotatedFrom string) (string, error) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return "", apierrors.InternalError("failed to generate refresh token")
	}
	if err := uc.refresh.Create(ctx, domain.RefreshToken{
		SessionID:   sessionID,
		ClientID:    clientID,
		TokenHash:   hash,
		Scopes:      scopes,
		RotatedFrom: rotatedFrom,
		ExpiresAt:   time.Now().UTC().Add(refreshTokenTTL),
	}); err != nil {
		return "", err
	}
	return raw, nil
}

// ClientCredentials issues an access token to a confidential client acting as its own principal (RFC 6749 §4.4).
func (uc *oauthUsecase) ClientCredentials(ctx context.Context, in ClientCredentialsInput) (TokenResponse, error) {
	client, err := uc.clients.Get(ctx, byRealmClientID(in.RealmID, in.ClientID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return TokenResponse{}, apierrors.Unauthenticated("unknown client")
		}
		return TokenResponse{}, err
	}
	if client.ClientType() != domain.ClientTypeConfidential {
		return TokenResponse{}, apierrors.Unauthenticated("client_credentials requires a confidential client")
	}
	if err := authenticateClient(client, in.ClientSecret); err != nil {
		return TokenResponse{}, err
	}
	if !slices.Contains(client.GrantTypes(), "client_credentials") {
		return TokenResponse{}, apierrors.InvalidArgument("client is not allowed the client_credentials grant")
	}

	scopes, err := resolveScopes(in.Scopes, client.Scopes())
	if err != nil {
		return TokenResponse{}, err
	}

	access, expiresIn, err := uc.tokens.MintAccessToken(ctx, in.RealmID, AccessTokenInput{
		Issuer: in.Issuer, Subject: in.ClientID, Audience: in.ClientID, ClientID: in.ClientID, Scopes: scopes, TTL: accessTokenTTL,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: expiresIn, Scope: strings.Join(scopes, " ")}, nil
}

// resolveScopes returns the requested scopes if they're a subset of allowed; defaults to allowed when none requested.
func resolveScopes(requested, allowed []string) ([]string, error) {
	if len(requested) == 0 {
		return allowed, nil
	}
	for _, s := range requested {
		if !slices.Contains(allowed, s) {
			return nil, apierrors.InvalidArgument("requested scope is not allowed for this client")
		}
	}
	return requested, nil
}

// Introspect reports whether a token is active and returns its claims (RFC 7662).
func (uc *oauthUsecase) Introspect(ctx context.Context, in IntrospectInput) (IntrospectionResult, error) {
	if err := uc.authenticateCaller(ctx, in.RealmID, in.ClientID, in.ClientSecret); err != nil {
		return IntrospectionResult{}, err
	}

	if claims, err := uc.tokens.VerifyAccessToken(ctx, in.RealmID, in.Token); err == nil {
		return IntrospectionResult{
			Active: true, TokenType: "Bearer", Scope: claims.Scope, ClientID: claims.ClientID,
			Subject: claims.Subject, Exp: claims.ExpiresAt, Iat: claims.IssuedAt,
			Audience: claims.Audience, Issuer: claims.Issuer,
			OrgID: claims.OrgID, OrgRole: claims.OrgRole,
		}, nil
	}

	rt, err := uc.refresh.GetByHash(ctx, hashToken(in.Token))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return IntrospectionResult{Active: false}, nil
		}
		return IntrospectionResult{}, err
	}
	now := time.Now().UTC()
	if rt.UsedAt != nil || !now.Before(rt.ExpiresAt) {
		return IntrospectionResult{Active: false}, nil
	}
	sess, err := uc.sessions.Get(ctx, byID(rt.SessionID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return IntrospectionResult{Active: false}, nil
		}
		return IntrospectionResult{}, err
	}
	if !domain.SessionActive(sess, now) {
		return IntrospectionResult{Active: false}, nil
	}
	return IntrospectionResult{
		Active: true, TokenType: "refresh_token", Scope: strings.Join(rt.Scopes, " "),
		ClientID: rt.ClientID, Subject: sess.AccountID(), Exp: rt.ExpiresAt.Unix(),
	}, nil
}

// Revoke kills the refresh-token chain by revoking the session (RFC 7009); access tokens are stateless and unknown tokens still succeed.
func (uc *oauthUsecase) Revoke(ctx context.Context, in RevokeInput) error {
	if err := uc.authenticateCaller(ctx, in.RealmID, in.ClientID, in.ClientSecret); err != nil {
		return err
	}
	rt, err := uc.refresh.GetByHash(ctx, hashToken(in.Token))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil
		}
		return err
	}
	return uc.sessions.Revoke(ctx, rt.SessionID, time.Now().UTC())
}

// authenticateCaller verifies the client behind an introspection/revocation request.
func (uc *oauthUsecase) authenticateCaller(ctx context.Context, realmID, clientID, secret string) error {
	client, err := uc.clients.Get(ctx, byRealmClientID(realmID, clientID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return apierrors.Unauthenticated("unknown client")
		}
		return err
	}
	return authenticateClient(client, secret)
}

// issueTokens mints the access token and, if the openid scope is present, the ID token.
func (uc *oauthUsecase) issueTokens(ctx context.Context, realmID, issuer, clientID, accountID string, scopes []string, nonce string) (TokenResponse, error) {
	orgID, orgRole, err := uc.orgs.ActiveOrg(ctx, accountID)
	if err != nil {
		return TokenResponse{}, err
	}
	access, expiresIn, err := uc.tokens.MintAccessToken(ctx, realmID, AccessTokenInput{
		Issuer: issuer, Subject: accountID, Audience: clientID, ClientID: clientID, Scopes: scopes,
		OrgID: orgID, OrgRole: orgRole, TTL: accessTokenTTL,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	resp := TokenResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: expiresIn, Scope: strings.Join(scopes, " ")}

	if slices.Contains(scopes, "openid") {
		acc, err := uc.accounts.Get(ctx, byID(accountID))
		if err != nil {
			return TokenResponse{}, err
		}
		idToken, err := uc.tokens.MintIDToken(ctx, realmID, IDTokenInput{
			Issuer: issuer, Subject: accountID, Audience: clientID, Nonce: nonce,
			Email: acc.Email(), EmailVerified: acc.EmailVerified(), Name: acc.DisplayName(), TTL: idTokenTTL,
		})
		if err != nil {
			return TokenResponse{}, err
		}
		resp.IDToken = idToken
	}
	return resp, nil
}

// authenticateClient checks the client secret for confidential clients; public clients pass through (PKCE covers them).
func authenticateClient(client domain.Client, secret string) error {
	if client.ClientType() != domain.ClientTypeConfidential {
		return nil
	}
	if secret == "" || hashToken(secret) != client.SecretHash() {
		return apierrors.Unauthenticated("invalid client credentials")
	}
	return nil
}

func byRealmClientID(realmID, clientID string) search.Option {
	return search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.ClientID, clientID),
	)
}
