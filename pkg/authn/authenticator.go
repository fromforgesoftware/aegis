package authn

import (
	"net/http"
	"strings"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

// Authenticator adapts a Verifier to kitrest's HTTPAuthenticator: it extracts
// the bearer token, verifies it, and injects the claims into the context.
type Authenticator struct {
	verifier *Verifier
}

func NewAuthenticator(v *Verifier) *Authenticator { return &Authenticator{verifier: v} }

func (a *Authenticator) Authenticate(r *http.Request) error {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return apierrors.Unauthorized("missing bearer token")
	}
	raw := strings.TrimPrefix(h, "Bearer ")
	claims, err := a.verifier.Verify(r.Context(), raw)
	if err != nil {
		return apierrors.Unauthorized("invalid token")
	}
	ctx := WithClaims(r.Context(), claims)
	ctx = WithRawToken(ctx, raw)
	*r = *r.WithContext(ctx)
	return nil
}
