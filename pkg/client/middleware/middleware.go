// Package middleware turns the Aegis client's token→account resolution into
// drop-in gRPC and HTTP server middleware: a consumer service adds it to its
// chain and downstream handlers read the resolved account from the context.
package middleware

import (
	"context"
	"strings"

	"github.com/fromforgesoftware/aegis/pkg/client"
)

// Resolver is the subset of the client the middleware needs.
type Resolver interface {
	Resolve(ctx context.Context, realmID, idpName, token string) (client.ResolvedAccount, error)
}

type accountIDKey struct{}

// WithAccountID stores the resolved account on the context.
func WithAccountID(ctx context.Context, accountID string) context.Context {
	return context.WithValue(ctx, accountIDKey{}, accountID)
}

// AccountIDFromContext returns the account the middleware resolved, if any.
func AccountIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(accountIDKey{}).(string)
	return id, ok && id != ""
}

type config struct {
	realmID    string
	idpName    string
	skip       map[string]bool
	bearerOnly bool
}

// Option configures the realm/IdP the middleware resolves against and which
// routes skip resolution.
type Option func(*config)

// WithRealm sets the realm the consumer federates against.
func WithRealm(realmID string) Option { return func(c *config) { c.realmID = realmID } }

// WithIDP sets the IdP name tokens are verified against.
func WithIDP(idpName string) Option { return func(c *config) { c.idpName = idpName } }

// Skip marks routes (gRPC full-method names or HTTP paths) that bypass
// resolution — health checks, login endpoints, etc.
func Skip(routes ...string) Option {
	return func(c *config) {
		for _, r := range routes {
			c.skip[r] = true
		}
	}
}

func newConfig(opts ...Option) config {
	c := config{skip: map[string]bool{}, bearerOnly: true}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// bearerToken strips a "Bearer " prefix; returns ok=false for an empty token.
func bearerToken(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	return raw, raw != ""
}
