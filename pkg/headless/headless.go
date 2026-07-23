// Package headless logs a user into an Aegis realm without a browser: it
// drives the hosted login form and the authorization_code + PKCE flow
// directly, returning a real realm-issued access token. It lives in the Aegis
// module because it is coupled to the login form's markup and routes — dev
// tooling and e2e suites use it (`trading-bot cli token`); production
// services never should.
package headless

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

// Config identifies the realm, the public PKCE client, and the user to log in.
type Config struct {
	BaseURL     string // e.g. http://127.0.0.1:18080
	Realm       string // e.g. trading-bot
	ClientID    string // a public PKCE OIDC client of the realm
	RedirectURI string // one of the client's registered redirect URIs
	Email       string
	Password    string
}

// Login performs the headless authorization_code + PKCE login and returns the
// access token.
func Login(ctx context.Context, cfg Config) (string, error) {
	c, err := newFlowClient(cfg.BaseURL, cfg.Realm)
	if err != nil {
		return "", err
	}
	return c.login(ctx, cfg.ClientID, cfg.RedirectURI, cfg.Email, cfg.Password)
}

// flowClient drives the login flow with a cookie jar (to carry the session
// cookie across the login redirects) that never auto-follows redirects, so
// each Location header can be read (the auth code arrives as a redirect that
// must not be followed).
type flowClient struct {
	base  string
	realm string
	http  *http.Client
}

// flowIDRe pulls the flow id out of aegis's rendered login form
// (<input type="hidden" name="flow" value="..." />).
var flowIDRe = regexp.MustCompile(`name="flow"\s+value="([^"]*)"`)

func newFlowClient(base, realm string) (*flowClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &flowClient{
		base:  strings.TrimRight(base, "/"),
		realm: realm,
		http: &http.Client{
			Jar:           jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (a *flowClient) login(ctx context.Context, clientID, redirectURI, email, password string) (string, error) {
	verifier, challenge := pkcePair()
	state := randToken()

	// The authorize request the login redirects back to once a session exists.
	// It must be a same-origin relative path (aegis blocks non-relative return_to).
	authzPath := "/realms/" + a.realm + "/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {"openid profile email"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	// 1. Fetch the login page to mint a flow id.
	loginURL := a.base + "/auth/login?" + url.Values{"realm": {a.realm}, "return_to": {authzPath}}.Encode()
	flowID, err := a.flowID(ctx, loginURL)
	if err != nil {
		return "", err
	}

	// 2. Submit credentials → establishes the session cookie and redirects to authzPath.
	loc, err := a.formRedirect(ctx, a.base+"/auth/login", url.Values{
		"flow":      {flowID},
		"email":     {email},
		"password":  {password},
		"return_to": {authzPath},
	})
	if err != nil {
		return "", fmt.Errorf("login as %s failed (check credentials): %w", email, err)
	}

	// 3. Follow the authorize redirect (now with a session) → redirect to redirect_uri?code=...
	codeLoc, err := a.getRedirect(ctx, a.base+loc)
	if err != nil {
		return "", fmt.Errorf("authorize after login: %w", err)
	}
	cu, err := url.Parse(codeLoc)
	if err != nil {
		return "", fmt.Errorf("parse callback %q: %w", codeLoc, err)
	}
	if got := cu.Query().Get("state"); got != state {
		return "", fmt.Errorf("oauth state mismatch (got %q)", got)
	}
	code := cu.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("no authorization code in callback %q", codeLoc)
	}

	// 4. Exchange the code for an access token.
	return a.tokenExchange(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	})
}

// flowID GETs the login page and extracts the flow id from the rendered form.
func (a *flowClient) flowID(ctx context.Context, loginURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET login page (is aegis reachable at %s?): %w", a.base, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login page: %s", status(resp))
	}
	html, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	m := flowIDRe.FindSubmatch(html)
	if m == nil {
		return "", fmt.Errorf("could not find login flow id (realm %q exists?)", a.realm)
	}
	return string(m[1]), nil
}

// formRedirect POSTs a urlencoded form and returns the Location of the expected
// redirect. A 200 means the form re-rendered (e.g. bad credentials).
func (a *flowClient) formRedirect(ctx context.Context, u string, form url.Values) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	loc := resp.Header.Get("Location")
	if !isRedirect(resp.StatusCode) || loc == "" {
		return "", fmt.Errorf("expected redirect, got %s", status(resp))
	}
	return loc, nil
}

// getRedirect GETs a URL and returns the Location of the expected redirect.
func (a *flowClient) getRedirect(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	loc := resp.Header.Get("Location")
	if !isRedirect(resp.StatusCode) || loc == "" {
		return "", fmt.Errorf("expected redirect, got %s", status(resp))
	}
	return loc, nil
}

// tokenExchange POSTs to the realm token endpoint and returns the access token.
func (a *flowClient) tokenExchange(ctx context.Context, form url.Values) (string, error) {
	u := a.base + "/realms/" + a.realm + "/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange: %s", status(resp))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token exchange returned no access_token")
	}
	return tok.AccessToken, nil
}

func pkcePair() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func randToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func isRedirect(code int) bool { return code >= 300 && code < 400 }

// status renders a short "<code>: <body excerpt>" for error messages.
func status(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		return resp.Status
	}
	return resp.Status + ": " + msg
}
