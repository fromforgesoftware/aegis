package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fromforgesoftware/go-kit/auth"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// PreferenceController serves the authenticated account's preferences.
//
//	GET    /api/me/preferences?keys=ui.theme,ui.locale   (auth: self)
//	PATCH  /api/me/preferences                            (auth: self)
//	DELETE /api/me/preferences?keys=ui.theme              (auth: self)
//	GET    /api/me/preferences/registry                   (auth: self)
//
// It lives under /api/me rather than /api/accounts/{id}/preferences because the
// only account whose preferences anyone may read or write here is their own. A
// path carrying an id invites the question "whose id may I pass?", and the answer
// would have to be "only your own" — enforced by comparing it to the token, which
// is what /api/me expresses without the comparison.
//
// `keys` is a comma-separated query parameter rather than JSON:API's
// `filter[key]=` because the client is a settings page asking for the six values
// it renders, and this reads plainly in a log and a browser address bar. A sparse
// read matters: without it the response grows with every preference ever added to
// the platform, on a request that happens on every page load.
type PreferenceController struct {
	preferences app.PreferenceUsecase
	realms      app.RealmUsecase
	tokens      app.TokenIssuer
}

func NewPreferenceController(
	preferences app.PreferenceUsecase,
	realms app.RealmUsecase,
	tokens app.TokenIssuer,
) kitrest.Controller {
	return &PreferenceController{preferences: preferences, realms: realms, tokens: tokens}
}

func (c *PreferenceController) Routes(r kitrest.Router) {
	r.Route("/api/me/preferences", func(r kitrest.Router) {
		r.Get("", c.requireSelf(http.HandlerFunc(c.list)))
		r.Patch("", c.requireSelf(http.HandlerFunc(c.update)))
		r.Delete("", c.requireSelf(http.HandlerFunc(c.reset)))
		// The registry lives under /api/me too, even though its content is identical
		// for every caller. A sibling path like /api/preferences/registry would be a
		// SECOND prefix every gateway in the platform has to learn — and the first
		// deployment proved it: /api/me was already routed and this was not, so the
		// endpoint 404'd at the gateway while the rest of the feature worked.
		// It stays behind a token because it enumerates the settings surface.
		r.Get("/registry", c.requireSelf(http.HandlerFunc(c.registry)))
	})

}

// requireSelf authenticates a realm end-user's bearer token, exactly as the avatar
// controller does for /api/me/avatar. Sharing the mechanism matters more than
// sharing the code: two different notions of "the current account" under the same
// /api/me prefix would be a real hazard.
func (c *PreferenceController) requireSelf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeJSONError(w, apierrors.Unauthorized("authentication required"))
			return
		}
		raw := strings.TrimPrefix(h, "Bearer ")
		tok, err := auth.NewToken(raw, auth.TokenType("Bearer"), nil)
		if err != nil {
			writeJSONError(w, apierrors.Unauthorized("invalid token"))
			return
		}
		name := realmNameFromIssuer(tok.Claims().Get("iss"))
		if name == "" {
			writeJSONError(w, apierrors.Unauthorized("token is not realm-scoped"))
			return
		}
		realm, err := c.realms.Get(r.Context(), app.RealmByName(name))
		if err != nil || realm == nil {
			writeJSONError(w, apierrors.Unauthorized("unknown realm"))
			return
		}
		if _, err := c.tokens.VerifyAccessToken(r.Context(), realm.ID(), raw); err != nil {
			writeJSONError(w, apierrors.Unauthorized("invalid token"))
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.InjectTokenInCtx(r.Context(), tok)))
	})
}

// accountID reads the subject of the verified token.
func accountIDFromCtx(r *http.Request) (string, bool) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		return "", false
	}
	return tok.Claims().Subject(), true
}

// requestedKeys parses ?keys=a,b,c. An absent parameter means the whole registry.
func requestedKeys(r *http.Request) []string {
	raw := strings.TrimSpace(r.URL.Query().Get("keys"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		if k := strings.TrimSpace(p); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

func (c *PreferenceController) list(w http.ResponseWriter, r *http.Request) {
	accountID, ok := accountIDFromCtx(r)
	if !ok {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	resolved, err := c.preferences.Resolve(r.Context(), accountID, requestedKeys(r))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writePreferences(w, resolved)
}

// updateRequest is a JSON:API-shaped bulk body: each member addresses a
// preference by its key as the resource id.
//
// PATCH rather than PUT because this is a partial update of the account's
// preference set — the keys absent from the body keep their current values. A PUT
// would have to mean "these are now all of my preferences", which would make a
// settings page that renders six controls delete every other preference on save.
type updateRequest struct {
	Data []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Value string `json:"value"`
		} `json:"attributes"`
	} `json:"data"`
}

func (c *PreferenceController) update(w http.ResponseWriter, r *http.Request) {
	accountID, ok := accountIDFromCtx(r)
	if !ok {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}

	var body updateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPreferenceBodyBytes)).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if len(body.Data) == 0 {
		writeJSONError(w, apierrors.InvalidArgument("data must contain at least one preference"))
		return
	}

	values := make(map[string]string, len(body.Data))
	for _, member := range body.Data {
		if member.Type != string(api.ResourceTypePreference) {
			writeJSONError(w, apierrors.InvalidArgument("resource type must be preferences"))
			return
		}
		if member.ID == "" {
			writeJSONError(w, apierrors.InvalidArgument("each preference needs an id (its key)"))
			return
		}
		// A body naming the same key twice has no defined outcome — the last write
		// would win by map iteration, which is not something a caller can predict.
		if _, duplicate := values[member.ID]; duplicate {
			writeJSONError(w, apierrors.InvalidArgument("preference "+member.ID+" appears twice"))
			return
		}
		values[member.ID] = member.Attributes.Value
	}

	if err := c.preferences.SetForAccount(r.Context(), accountID, values); err != nil {
		writeJSONError(w, err)
		return
	}

	// The updated set is returned rather than a 204, so the client's store is
	// reconciled from what the server actually resolved — including the case where
	// a workspace-administered key means the stored value is not the effective one.
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	resolved, err := c.preferences.Resolve(r.Context(), accountID, keys)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writePreferences(w, resolved)
}

// writePreferences renders a resolved set as a JSON:API collection.
//
// jsonapi.MarshalManyPayloads, not encoding/json: the DTO describes itself with
// `jsonapi:"attr,…"` tags, which encoding/json ignores entirely — so marshalling it
// the obvious way produced {"RKey":…,"RValue":…} instead of the attributes object,
// and no JSON:API client could read it.
func writePreferences(w http.ResponseWriter, resolved []domain.Preference) {
	list := resource.ListResponseToDTO(api.PreferenceToDTO)(
		resource.NewListResponse(resolved, len(resolved)))
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

func (c *PreferenceController) reset(w http.ResponseWriter, r *http.Request) {
	accountID, ok := accountIDFromCtx(r)
	if !ok {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	keys := requestedKeys(r)
	if len(keys) == 0 {
		// Refusing an empty keys list is deliberate: the obvious reading of DELETE
		// with no filter is "reset everything", and silently wiping every
		// preference on a request that forgot its query string is not a mistake to
		// make easy.
		writeJSONError(w, apierrors.InvalidArgument("keys is required; name the preferences to reset"))
		return
	}
	if err := c.preferences.ResetForAccount(r.Context(), accountID, keys); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *PreferenceController) registry(w http.ResponseWriter, r *http.Request) {
	specs := domain.PreferenceRegistry()
	list := resource.ListResponseToDTO(api.PreferenceSpecToDTO)(
		resource.NewListResponse(specs, len(specs)))
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalManyPayloads(w, list)
}

// maxPreferenceBodyBytes bounds a batch update. The registry cap and per-value
// limits are the real constraints; this only stops an unbounded read, as the
// avatar controller's reader does.
const maxPreferenceBodyBytes = 64 << 10
