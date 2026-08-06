package http

import (
	"context"
	"net/http"

	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
)

// writeAPIError renders err as a JSON:API error document, preserving its status, code, title and
// human-readable detail.
//
// The self-service controllers use this instead of writeJSONError, which exists for the OAuth and
// OIDC endpoints: RFC 6749 §5.2 fixes the set of values their `error` field may take, so those
// responses deliberately say "server_error" and nothing more. Reusing that on a resource endpoint
// threw away every message the handler had bothered to write — a rejected password, a duplicate
// workspace slug and an unparseable body all reached the browser as {"error":"server_error"},
// leaving a settings form with nothing to show but its own guess.
//
// This is the same encoder the kit's generated JSON:API handlers use, so a hand-written handler and
// a generated one on the same resource report failures identically.
func writeAPIError(ctx context.Context, w http.ResponseWriter, err error) {
	kitrest.JsonApiErrorEncoder(ctx, err, w)
}
