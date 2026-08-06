package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fromforgesoftware/go-kit/auth"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// ProfileController serves the signed-in account's own details.
//
//	GET   /api/me           (auth: self)
//	PATCH /api/me           (auth: self)
//	PUT   /api/me/password  (auth: self)
//
// Under /api/me alongside the avatar and preference endpoints, and for the same reason: the
// only account any of this may touch is the caller's own, which a path carrying an id would
// invite questioning.
//
// A settings page previously had to read the name and email out of the JWT, because aegis
// exposed no way to ask. That works until the stored value changes — a token is issued once
// and cached, so the form would keep showing the old name until it expired, including
// immediately after the user edited it.
type ProfileController struct {
	profiles app.ProfileUsecase
	realms   app.RealmUsecase
	tokens   app.TokenIssuer
}

func NewProfileController(
	profiles app.ProfileUsecase, realms app.RealmUsecase, tokens app.TokenIssuer,
) kitrest.Controller {
	return &ProfileController{profiles: profiles, realms: realms, tokens: tokens}
}

func (c *ProfileController) Routes(r kitrest.Router) {
	r.Get("/api/me", c.requireSelf(http.HandlerFunc(c.read)))
	r.Patch("/api/me", c.requireSelf(http.HandlerFunc(c.update)))
	r.Put("/api/me/password", c.requireSelf(http.HandlerFunc(c.changePassword)))
}

// requireSelf authenticates a realm end-user's bearer token, as the avatar and preference
// controllers do. Sharing the mechanism matters more than sharing the code: two different
// notions of "the current account" under one /api/me prefix would be a real hazard.
func (c *ProfileController) requireSelf(next http.Handler) http.Handler {
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

func subjectFromCtx(r *http.Request) (string, bool) {
	tok := auth.TokenFromCtx(r.Context())
	if tok == nil {
		return "", false
	}
	return tok.Claims().Subject(), true
}

func (c *ProfileController) read(w http.ResponseWriter, r *http.Request) {
	accountID, ok := subjectFromCtx(r)
	if !ok {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}
	acc, err := c.profiles.Profile(r.Context(), accountID)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalPayload(w, api.ProfileToDTO(acc))
}

// updateRequest is the PATCH body.
//
// The name fields are POINTERS so an omitted field is distinguishable from one sent empty.
// With plain strings a PATCH carrying only the given name would clear the family name, which
// makes every partial update destructive — the exact failure PATCH exists to avoid.
type profileUpdateRequest struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			GivenName  *string `json:"givenName"`
			FamilyName *string `json:"familyName"`
		} `json:"attributes"`
	} `json:"data"`
}

func (c *ProfileController) update(w http.ResponseWriter, r *http.Request) {
	accountID, ok := subjectFromCtx(r)
	if !ok {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}

	var body profileUpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxProfileBodyBytes)).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if body.Data.Type != "" && body.Data.Type != string(api.ResourceTypeProfile) {
		writeJSONError(w, apierrors.InvalidArgument("resource type must be profiles"))
		return
	}

	acc, err := c.profiles.UpdateProfile(r.Context(), app.UpdateProfileInput{
		AccountID:  accountID,
		GivenName:  body.Data.Attributes.GivenName,
		FamilyName: body.Data.Attributes.FamilyName,
	})
	if err != nil {
		writeJSONError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = jsonapi.MarshalPayload(w, api.ProfileToDTO(acc))
}

// passwordRequest is plain JSON rather than a JSON:API document.
//
// A password is not a resource: there is nothing to address, nothing to return, and no id to
// carry. Dressing it as one would mean inventing a "passwords" type whose only member is
// write-only.
type passwordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	// Confirm is accepted and checked here rather than in the usecase: it is a UI concern —
	// protection against a typo the user cannot see — and it has no meaning to a non-browser
	// caller. Omitting it is allowed; sending a mismatch is not.
	Confirm string `json:"confirm"`
}

func (c *ProfileController) changePassword(w http.ResponseWriter, r *http.Request) {
	accountID, ok := subjectFromCtx(r)
	if !ok {
		writeJSONError(w, apierrors.Unauthorized("authentication required"))
		return
	}

	var body passwordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxProfileBodyBytes)).Decode(&body); err != nil {
		writeJSONError(w, apierrors.InvalidArgument("invalid request body"))
		return
	}
	if body.Confirm != "" && body.Confirm != body.NewPassword {
		writeJSONError(w, apierrors.InvalidArgument("the passwords do not match"))
		return
	}

	if err := c.profiles.ChangePassword(r.Context(), app.ChangePasswordInput{
		AccountID:       accountID,
		CurrentPassword: body.CurrentPassword,
		NewPassword:     body.NewPassword,
	}); err != nil {
		writeJSONError(w, err)
		return
	}
	// 204 with no body. Returning anything here risks echoing part of what was just sent, and
	// there is nothing a client needs back beyond "it worked".
	w.WriteHeader(http.StatusNoContent)
}

// maxProfileBodyBytes bounds the request body. Two names and two passwords need very little;
// this only stops an unbounded read, as the avatar controller's reader does.
const maxProfileBodyBytes = 16 << 10
