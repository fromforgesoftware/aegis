package authn

import "context"

type claimsKey struct{}
type rawTokenKey struct{}

// WithClaims stores verified claims on the context.
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromCtx returns the verified claims, if the request was authenticated.
func ClaimsFromCtx(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(Claims)
	return c, ok
}

// WithRawToken stores the raw bearer token, so a service can forward it on
// service-to-service calls (token passthrough).
func WithRawToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, rawTokenKey{}, token)
}

// RawTokenFromCtx returns the raw bearer token of the authenticated request.
func RawTokenFromCtx(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(rawTokenKey{}).(string)
	return t, ok
}

// OwnerFromCtx is the opaque owner key services scope data by and pass to
// other forge services: the active org when the token carries one, else the
// subject.
func OwnerFromCtx(ctx context.Context) string {
	c, _ := ClaimsFromCtx(ctx)
	if c.OrgID != "" {
		return c.OrgID
	}
	return c.Subject
}

// OrgIDFromCtx returns the caller's active organization (the token's org_id),
// or "" when the token has none. The org is data, not an access gate —
// authorization is aegis ReBAC per resource — so this never fails closed.
func OrgIDFromCtx(ctx context.Context) string {
	c, ok := ClaimsFromCtx(ctx)
	if !ok {
		return ""
	}
	return c.OrgID
}
