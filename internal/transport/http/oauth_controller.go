package http

import (
	"net/http"
	"net/url"
	"strings"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/app"
)

// sessionCookie carries the hosted-login session id that /authorize authenticates against.
const sessionCookie = "aegis_session"

// OAuthController serves the per-realm OAuth2/OIDC protocol endpoints — RFC-shaped, not JSON:API.
type OAuthController struct {
	oauth  app.OAuthUsecase
	realms app.RealmUsecase
}

func NewOAuthController(oauth app.OAuthUsecase, realms app.RealmUsecase) kitrest.Controller {
	return &OAuthController{oauth: oauth, realms: realms}
}

// resolveRealm maps the {realm} URL path (a realm name, like Keycloak) to the
// realm's UUID, which every realm-scoped persistence row keys on.
func (c *OAuthController) resolveRealm(r *http.Request) (string, error) {
	realm, err := c.realms.Get(r.Context(), app.RealmByName(r.PathValue("realm")))
	if err != nil {
		return "", err
	}
	return realm.ID(), nil
}

func (c *OAuthController) Routes(r kitrest.Router) {
	r.Get("/realms/{realm}/authorize", http.HandlerFunc(c.authorize))
	r.Get("/realms/{realm}/logout", http.HandlerFunc(c.logout))
	r.Post("/realms/{realm}/token", http.HandlerFunc(c.token))
	r.Post("/realms/{realm}/introspect", http.HandlerFunc(c.introspect))
	r.Post("/realms/{realm}/revoke", http.HandlerFunc(c.revoke))
}

// logout is the OIDC RP-initiated logout (end_session_endpoint): it revokes the
// browser session, clears the cookie, and redirects to post_logout_redirect_uri.
func (c *OAuthController) logout(w http.ResponseWriter, r *http.Request) {
	if ck, err := r.Cookie(sessionCookie); err == nil {
		_ = c.oauth.EndSession(r.Context(), ck.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: requestScheme(r) == "https", SameSite: http.SameSiteLaxMode,
	})
	// NOTE: production should validate post_logout_redirect_uri against the
	// client's registered URIs (OIDC RP-initiated logout) to avoid open redirects.
	dest := r.URL.Query().Get("post_logout_redirect_uri")
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (c *OAuthController) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("response_type") != "code" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_response_type", "only response_type=code is supported")
		return
	}
	realmID, err := c.resolveRealm(r)
	if err != nil {
		writeOAuthError(w, oauthStatus(err), "invalid_request", "unknown realm")
		return
	}

	accountID, sessionID, ok := c.authenticatedAccount(r, realmID)
	if !ok {
		login := "/auth/login?realm=" + url.QueryEscape(r.PathValue("realm")) + "&return_to=" + url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, login, http.StatusFound)
		return
	}

	res, err := c.oauth.Authorize(r.Context(), app.AuthorizeInput{
		RealmID:             realmID,
		ClientID:            q.Get("client_id"),
		AccountID:           accountID,
		SessionID:           sessionID,
		RedirectURI:         q.Get("redirect_uri"),
		Scopes:              strings.Fields(q.Get("scope")),
		State:               q.Get("state"),
		Nonce:               q.Get("nonce"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
	})
	if err != nil {
		writeOAuthError(w, oauthStatus(err), "invalid_request", err.Error())
		return
	}

	u, err := url.Parse(res.RedirectURI)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid redirect_uri")
		return
	}
	rq := u.Query()
	rq.Set("code", res.Code)
	if res.State != "" {
		rq.Set("state", res.State)
	}
	u.RawQuery = rq.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (c *OAuthController) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	realm, err := c.resolveRealm(r)
	if err != nil {
		writeOAuthError(w, oauthStatus(err), "invalid_request", "unknown realm")
		return
	}
	clientID, clientSecret := clientAuth(r)

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		resp, err := c.oauth.ExchangeCode(r.Context(), app.CodeExchangeInput{
			RealmID:      realm,
			Issuer:       issuerURL(r),
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Code:         r.PostForm.Get("code"),
			RedirectURI:  r.PostForm.Get("redirect_uri"),
			CodeVerifier: r.PostForm.Get("code_verifier"),
		})
		if err != nil {
			writeOAuthError(w, oauthStatus(err), oauthErrorCode(err), err.Error())
			return
		}
		writeTokenResponse(w, resp)
	case "client_credentials":
		resp, err := c.oauth.ClientCredentials(r.Context(), app.ClientCredentialsInput{
			RealmID:      realm,
			Issuer:       issuerURL(r),
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       strings.Fields(r.PostForm.Get("scope")),
		})
		if err != nil {
			writeOAuthError(w, oauthStatus(err), oauthErrorCode(err), err.Error())
			return
		}
		writeTokenResponse(w, resp)
	case "refresh_token":
		resp, err := c.oauth.Refresh(r.Context(), app.RefreshInput{
			RealmID:      realm,
			Issuer:       issuerURL(r),
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RefreshToken: r.PostForm.Get("refresh_token"),
		})
		if err != nil {
			writeOAuthError(w, oauthStatus(err), oauthErrorCode(err), err.Error())
			return
		}
		writeTokenResponse(w, resp)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported grant_type")
	}
}

func (c *OAuthController) introspect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	realm, err := c.resolveRealm(r)
	if err != nil {
		writeOAuthError(w, oauthStatus(err), "invalid_request", "unknown realm")
		return
	}
	clientID, clientSecret := clientAuth(r)
	res, err := c.oauth.Introspect(r.Context(), app.IntrospectInput{
		RealmID:       realm,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		Token:         r.PostForm.Get("token"),
		TokenTypeHint: r.PostForm.Get("token_type_hint"),
	})
	if err != nil {
		writeOAuthError(w, oauthStatus(err), oauthErrorCode(err), err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, introspectionDTO(res))
}

func (c *OAuthController) revoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	realm, err := c.resolveRealm(r)
	if err != nil {
		writeOAuthError(w, oauthStatus(err), "invalid_request", "unknown realm")
		return
	}
	clientID, clientSecret := clientAuth(r)
	if err := c.oauth.Revoke(r.Context(), app.RevokeInput{
		RealmID:       realm,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		Token:         r.PostForm.Get("token"),
		TokenTypeHint: r.PostForm.Get("token_type_hint"),
	}); err != nil {
		writeOAuthError(w, oauthStatus(err), oauthErrorCode(err), err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

func (c *OAuthController) authenticatedAccount(r *http.Request, realm string) (accountID, sessionID string, ok bool) {
	ck, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", "", false
	}
	sess, err := c.oauth.ResolveSession(r.Context(), ck.Value)
	if err != nil || sess.RealmID() != realm {
		return "", "", false
	}
	return sess.AccountID(), sess.ID(), true
}

// clientAuth reads client credentials from HTTP Basic (preferred) or the request body.
func clientAuth(r *http.Request) (id, secret string) {
	if cid, csec, ok := r.BasicAuth(); ok {
		return cid, csec
	}
	return r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
}

type tokenResponseDTO struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func writeTokenResponse(w http.ResponseWriter, resp app.TokenResponse) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, tokenResponseDTO{
		AccessToken:  resp.AccessToken,
		TokenType:    resp.TokenType,
		ExpiresIn:    resp.ExpiresIn,
		RefreshToken: resp.RefreshToken,
		IDToken:      resp.IDToken,
		Scope:        resp.Scope,
	})
}

// introspectionResponseDTO is the RFC 7662 response; an inactive token emits only {"active": false}.
type introspectionResponseDTO struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Sub       string `json:"sub,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Aud       string `json:"aud,omitempty"`
	Iss       string `json:"iss,omitempty"`
}

func introspectionDTO(res app.IntrospectionResult) introspectionResponseDTO {
	return introspectionResponseDTO{
		Active:    res.Active,
		Scope:     res.Scope,
		ClientID:  res.ClientID,
		Sub:       res.Subject,
		TokenType: res.TokenType,
		Exp:       res.Exp,
		Iat:       res.Iat,
		Aud:       res.Audience,
		Iss:       res.Issuer,
	}
}

func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func oauthStatus(err error) int {
	if s := apierrors.GetHTTPStatus(err); s != 0 {
		return s
	}
	return http.StatusBadRequest
}

func oauthErrorCode(err error) string {
	switch {
	case apierrors.Is(err, apierrors.CodeUnauthenticated):
		return "invalid_client"
	case apierrors.Is(err, apierrors.CodeInvalidArgument):
		return "invalid_grant"
	default:
		return "server_error"
	}
}
