package client

import (
	"context"
	"net/http"
)

// AuthorizationAdminAPI maintains the authorization projection. Refresh makes
// prior writes (resources, bindings, roles, permissions) visible to Check;
// Sweep deletes expired bindings and refreshes, returning how many grants
// were removed.
type AuthorizationAdminAPI interface {
	Refresh(ctx context.Context) error
	Sweep(ctx context.Context) (int64, error)
}

// ------------------------------------------------------------ HTTP

type authorizationAdminHTTPClient struct {
	rest *restClient
}

func NewAuthorizationAdminHTTPClient(rest *restClient) *authorizationAdminHTTPClient {
	return &authorizationAdminHTTPClient{rest: rest}
}

func (c *authorizationAdminHTTPClient) Refresh(ctx context.Context) error {
	return restJSON(ctx, c.rest, http.MethodPost, "/api/authorizations/refresh", nil, nil)
}

type sweepResponse struct {
	Removed int64 `json:"removed"`
}

func (c *authorizationAdminHTTPClient) Sweep(ctx context.Context) (int64, error) {
	var resp sweepResponse
	if err := restJSON(ctx, c.rest, http.MethodPost, "/api/authorizations/sweep", nil, &resp); err != nil {
		return 0, err
	}
	return resp.Removed, nil
}
