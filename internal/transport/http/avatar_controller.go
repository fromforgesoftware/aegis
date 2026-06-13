package http

import (
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"

	"github.com/fromforgesoftware/go-kit/auth"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

// maxUploadBytes bounds the request body read. The usecase re-checks the exact
// limit; this just stops an unbounded read.
const maxUploadBytes = (1 << 20) + 1024

// AvatarController serves account avatars and organization logos.
//
//	POST   /api/me/avatar                      (auth: self)
//	DELETE /api/me/avatar                      (auth: self)
//	GET    /api/avatars/accounts/{id}          (public)
//	POST   /api/avatars/organizations/{id}     (auth: org owner)
//	DELETE /api/avatars/organizations/{id}     (auth: org owner)
//	GET    /api/avatars/organizations/{id}     (public)
//
// Serve endpoints are public and unauthenticated so plain <img src> tags work
// (avatars are low-sensitivity); mutations require a realm token. They live under
// a dedicated /api/avatars prefix to avoid colliding with the /api/accounts and
// /api/organizations route subtrees owned by other controllers.
type AvatarController struct {
	avatars app.AvatarUsecase
	orgs    app.OrganizationUsecase
	realms  app.RealmUsecase
	tokens  app.TokenIssuer
}

func NewAvatarController(
	avatars app.AvatarUsecase,
	orgs app.OrganizationUsecase,
	realms app.RealmUsecase,
	tokens app.TokenIssuer,
) kitrest.Controller {
	return &AvatarController{avatars: avatars, orgs: orgs, realms: realms, tokens: tokens}
}

func (c *AvatarController) Routes(r kitrest.Router) {
	r.Post("/api/me/avatar", c.requireRealmToken(http.HandlerFunc(c.uploadMyAvatar)))
	r.Delete("/api/me/avatar", c.requireRealmToken(http.HandlerFunc(c.deleteMyAvatar)))
	r.Get("/api/avatars/accounts/{id}", http.HandlerFunc(c.serveAccountAvatar))

	r.Post("/api/avatars/organizations/{id}", c.requireRealmToken(http.HandlerFunc(c.uploadOrgLogo)))
	r.Delete("/api/avatars/organizations/{id}", c.requireRealmToken(http.HandlerFunc(c.deleteOrgLogo)))
	r.Get("/api/avatars/organizations/{id}", http.HandlerFunc(c.serveOrgLogo))
}

// requireRealmToken authenticates a realm end-user's bearer token (same pattern
// as the organization controller) and injects it into the context.
func (c *AvatarController) requireRealmToken(next http.Handler) http.Handler {
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

func readImage(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxUploadBytes))
	if err != nil {
		writeJSONError(w, apierrors.InvalidArgument("could not read image (max 1 MiB)"))
		return nil, false
	}
	return body, true
}

func (c *AvatarController) uploadMyAvatar(w http.ResponseWriter, r *http.Request) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	image, ok := readImage(w, r)
	if !ok {
		return
	}
	if err := c.avatars.SetAccountAvatar(r.Context(), tok.Claims().Subject(), image); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *AvatarController) deleteMyAvatar(w http.ResponseWriter, r *http.Request) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	if err := c.avatars.DeleteAccountAvatar(r.Context(), tok.Claims().Subject()); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *AvatarController) serveAccountAvatar(w http.ResponseWriter, r *http.Request) {
	image, ct, found, err := c.avatars.GetAccountAvatar(r.Context(), r.PathValue("id"))
	serveImage(w, r, image, ct, found, err)
}

// requireOrgOwner returns the org-owner-validated org id, or writes an error and
// returns ok=false. Only the organization's owner may change its logo.
func (c *AvatarController) requireOrgOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return "", false
	}
	orgID := r.PathValue("id")
	org, err := c.orgs.Get(r.Context(), search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, orgID)))
	if err != nil || org == nil {
		writeJSONError(w, apierrors.NotFound("organization", orgID))
		return "", false
	}
	owner := org.Owner()
	if owner == nil || owner.ID() != tok.Claims().Subject() {
		writeJSONError(w, apierrors.Forbidden("only the workspace owner can change the logo"))
		return "", false
	}
	return orgID, true
}

func (c *AvatarController) uploadOrgLogo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := c.requireOrgOwner(w, r)
	if !ok {
		return
	}
	image, ok := readImage(w, r)
	if !ok {
		return
	}
	if err := c.avatars.SetOrgLogo(r.Context(), orgID, image); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *AvatarController) deleteOrgLogo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := c.requireOrgOwner(w, r)
	if !ok {
		return
	}
	if err := c.avatars.DeleteOrgLogo(r.Context(), orgID); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *AvatarController) serveOrgLogo(w http.ResponseWriter, r *http.Request) {
	image, ct, found, err := c.avatars.GetOrgLogo(r.Context(), r.PathValue("id"))
	serveImage(w, r, image, ct, found, err)
}

// serveImage writes the image with caching headers, honoring conditional
// requests via a content-hash ETag. A missing image is a 404.
func serveImage(w http.ResponseWriter, r *http.Request, image []byte, contentType string, found bool, err error) {
	if err != nil {
		writeJSONError(w, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	etag := fmt.Sprintf("\"%08x\"", crc32.ChecksumIEEE(image))
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image)
}
