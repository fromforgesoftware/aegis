package http

import (
	"encoding/json"
	"net/http"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/app"
)

// OIDCController serves the per-realm discovery + JWKS endpoints — RFC-shaped, not JSON:API.
type OIDCController struct {
	keys   app.SigningKeyService
	realms app.RealmUsecase
}

func NewOIDCController(keys app.SigningKeyService, realms app.RealmUsecase) kitrest.Controller {
	return &OIDCController{keys: keys, realms: realms}
}

func (c *OIDCController) Routes(r kitrest.Router) {
	r.Get("/realms/{realm}/.well-known/openid-configuration", http.HandlerFunc(c.discovery))
	r.Get("/realms/{realm}/.well-known/jwks.json", http.HandlerFunc(c.jwks))
}

// discoveryDocument is the RFC 8414 / OIDC Discovery metadata for a realm.
type discoveryDocument struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

func (c *OIDCController) discovery(w http.ResponseWriter, r *http.Request) {
	issuer := issuerURL(r)
	writeJSON(w, http.StatusOK, discoveryDoc(issuer))
}

func (c *OIDCController) jwks(w http.ResponseWriter, r *http.Request) {
	realm, err := c.realms.Get(r.Context(), app.RealmByName(r.PathValue("realm")))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	set, err := c.keys.JWKS(r.Context(), realm.ID())
	if err != nil {
		writeJSONError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(set)
}

func discoveryDoc(issuer string) discoveryDocument {
	return discoveryDocument{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + "/authorize",
		TokenEndpoint:                     issuer + "/token",
		IntrospectionEndpoint:             issuer + "/introspect",
		RevocationEndpoint:                issuer + "/revoke",
		EndSessionEndpoint:                issuer + "/logout",
		UserinfoEndpoint:                  issuer + "/userinfo",
		JWKSURI:                           issuer + "/.well-known/jwks.json",
		ResponseTypesSupported:            []string{"code"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		GrantTypesSupported:               []string{"authorization_code", "client_credentials", "refresh_token"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post", "none"},
		ScopesSupported:                   []string{"openid", "profile", "email"},
		CodeChallengeMethodsSupported:     []string{"S256"},
	}
}

// issuerURL builds the realm's issuer, honouring a TLS-terminating proxy's forwarded scheme/host.
func issuerURL(r *http.Request) string {
	return requestScheme(r) + "://" + requestHost(r) + "/realms/" + r.PathValue("realm")
}

func requestScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return host
	}
	return r.Host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, err error) {
	status := apierrors.GetHTTPStatus(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{"error": "server_error"})
}
