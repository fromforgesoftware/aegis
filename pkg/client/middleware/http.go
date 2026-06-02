package middleware

import (
	"net/http"
)

// HTTP resolves the Authorization bearer token to an account and injects it
// into the request context for downstream handlers. Skipped paths and requests
// without a token (on non-skipped paths) get 401.
func HTTP(r Resolver, opts ...Option) func(http.Handler) http.Handler {
	cfg := newConfig(opts...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if cfg.skip[req.URL.Path] {
				next.ServeHTTP(w, req)
				return
			}
			token, ok := bearerToken(req.Header.Get("Authorization"))
			if !ok {
				http.Error(w, "missing identity token", http.StatusUnauthorized)
				return
			}
			acct, err := r.Resolve(req.Context(), cfg.realmID, cfg.idpName, token)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, req.WithContext(WithAccountID(req.Context(), acct.AccountID)))
		})
	}
}
