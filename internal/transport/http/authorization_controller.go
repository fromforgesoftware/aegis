package http

import (
	"context"
	"net/http"
	"strconv"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// AuthorizationController exposes the hot-path authorization reads plus
// projection maintenance. The gRPC AuthorizerService is the primary S2S
// surface; these REST endpoints serve browser/admin callers.
type AuthorizationController struct {
	authz   app.AuthorizationUsecase
	sweeper app.GrantSweeper
}

func NewAuthorizationController(authz app.AuthorizationUsecase, sweeper app.GrantSweeper) kitrest.Controller {
	return &AuthorizationController{authz: authz, sweeper: sweeper}
}

func (c *AuthorizationController) Routes(r kitrest.Router) {
	r.Route("/api/authorizations", func(r kitrest.Router) {
		r.Post("/refresh", http.HandlerFunc(c.refresh))
		r.Post("/sweep", http.HandlerFunc(c.sweep))
		r.Post("/check", kitrest.NewJsonApiCommandHandler(
			c.check, decodeCheck, identityCheckDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Check a permission"),
				openapi.Description("Returns allowed=true when the account holds the permission on the resource."),
				openapi.Tags("authz"), openapi.Errors(400),
			),
		))
		r.Post("/batch-check", kitrest.NewJsonApiCommandHandler(
			c.batchCheck, decodeBatchCheck, identityBatchCheckDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Check many permissions for one account"),
				openapi.Tags("authz"), openapi.Errors(400),
			),
		))
		r.Get("/accessible", http.HandlerFunc(c.listAccessible))
		r.Get("/version", kitrest.NewJsonApiCommandHandler(
			c.version, decodeEmptyVersion, identityVersionDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(
				openapi.Summary("Read the authorization versions"),
				openapi.Description("Returns writeVersion (latest mutation) and projectionVersion (what Check currently reflects)."),
				openapi.Tags("authz"),
			),
		))
	})
}

// refresh rebuilds the effective_authorizations materialised view so writes
// since the last refresh become visible to Check.
func (c *AuthorizationController) refresh(w http.ResponseWriter, r *http.Request) {
	if err := c.authz.Refresh(r.Context()); err != nil {
		writeJSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type sweepResponse struct {
	Removed int64 `json:"removed"`
}

// sweep deletes expired bindings and refreshes the projection, returning how
// many grants were removed.
func (c *AuthorizationController) sweep(w http.ResponseWriter, r *http.Request) {
	removed, err := c.sweeper.Sweep(r.Context())
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sweepResponse{Removed: removed})
}

func (c *AuthorizationController) check(ctx context.Context, in api.CheckRequestDTO) (*api.CheckDTO, error) {
	allowed, err := c.authz.Check(ctx, in.RAccountID, in.RResourceID, in.RPermissionID, in.RMinVersion)
	if err != nil {
		return nil, err
	}
	return api.CheckToDTO(allowed), nil
}

func (c *AuthorizationController) version(ctx context.Context, _ versionQuery) (*api.VersionDTO, error) {
	write, projection, err := c.authz.Version(ctx)
	if err != nil {
		return nil, err
	}
	return api.VersionToDTO(write, projection), nil
}

type versionQuery struct{}

func decodeEmptyVersion(*http.Request) (versionQuery, error) { return versionQuery{}, nil }

func identityVersionDTO(dto *api.VersionDTO) *api.VersionDTO { return dto }

func decodeCheck(req *http.Request) (api.CheckRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.CheckRequestDTO](req)
	if err != nil {
		return api.CheckRequestDTO{}, err
	}
	return *body, nil
}

func identityCheckDTO(dto *api.CheckDTO) *api.CheckDTO { return dto }

func (c *AuthorizationController) batchCheck(ctx context.Context, in api.BatchCheckRequestDTO) (*api.BatchCheckDTO, error) {
	decisions, err := c.authz.BatchCheck(ctx, in.RAccountID, api.ChecksFromDTO(in.RChecks), in.RMinVersion)
	if err != nil {
		return nil, err
	}
	return api.BatchCheckToDTO(decisions), nil
}

func decodeBatchCheck(req *http.Request) (api.BatchCheckRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.BatchCheckRequestDTO](req)
	if err != nil {
		return api.BatchCheckRequestDTO{}, err
	}
	return *body, nil
}

func identityBatchCheckDTO(dto *api.BatchCheckDTO) *api.BatchCheckDTO { return dto }

// parseMinVersion reads the optional minVersion query param; absent means no
// freshness constraint.
func parseMinVersion(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, apierrors.InvalidArgument("minVersion must be an integer")
	}
	return v, nil
}

type accessibleResourcesResponse struct {
	Data []accessibleResourceRef `json:"data"`
}

type accessibleResourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// listAccessible returns every resource the account holds the permission on as
// a JSON:API resource-identifier document.
func (c *AuthorizationController) listAccessible(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("accountId")
	permissionID := r.URL.Query().Get("permissionId")
	if accountID == "" || permissionID == "" {
		writeJSONError(w, apierrors.InvalidArgument("accountId and permissionId query params are required"))
		return
	}
	minVersion, err := parseMinVersion(r.URL.Query().Get("minVersion"))
	if err != nil {
		writeJSONError(w, err)
		return
	}
	ids, err := c.authz.ListAccessible(r.Context(), accountID, permissionID, minVersion)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	refs := make([]accessibleResourceRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, accessibleResourceRef{Type: string(api.ResourceTypeAuthzResource), ID: id})
	}
	writeJSON(w, http.StatusOK, accessibleResourcesResponse{Data: refs})
}
