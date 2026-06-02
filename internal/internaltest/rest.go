package internaltest

import (
	"net/http"

	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
)

// NewRESTHandler wraps a kit Controller with the same routing the
// production server uses, without the http.Server — so transport tests
// exercise exactly the routes production wires.
func NewRESTHandler(c kitrest.Controller) http.Handler {
	return kitrest.BuildHandler(c)
}
