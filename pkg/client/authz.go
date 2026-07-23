package client

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/pkg/authn"
)

// Authz is the single-call authorization surface for consumer services: one
// call per intent, identity derived from the pkg/authn claims already in the
// request ctx, errors returned as ready-to-serve go-kit apierrors (401/403) —
// services never translate them. It owns the choreography (owner registration,
// idempotency, projection refresh) so a service's whole authz integration is
// Register on create, Unregister on delete, Authorize in guards, and
// AccessibleIDs for lists.
//
// The wire trust model (today: explicit fields the server trusts; later:
// forwarded caller token, server-derived identity) is an implementation detail
// behind this type — call sites won't change when it does.
type Authz struct {
	c Client
}

// NewAuthz wraps a Client with the single-call surface.
func NewAuthz(c Client) *Authz {
	return &Authz{c: c}
}

// RegisterOption tweaks a Register call.
type RegisterOption func(*Resource)

// WithParent parents the resource for grant inheritance.
func WithParent(parentID string) RegisterOption {
	return func(r *Resource) { r.ParentID = parentID }
}

// WithVisibility overrides the PRIVATE default.
func WithVisibility(v string) RegisterOption {
	return func(r *Resource) { r.Visibility = v }
}

// WithOwner overrides the claims-derived owner — for tooling (fixtures,
// seeders) acting on behalf of an account rather than as one.
func WithOwner(accountID string) RegisterOption {
	return func(r *Resource) { r.OwnerAccountID = accountID }
}

// WithRealm overrides the claims-derived realm — pair with WithOwner in
// tooling contexts that have no user ctx.
func WithRealm(realmID string) RegisterOption {
	return func(r *Resource) { r.RealmID = realmID }
}

// Register records a domain object as an authz resource UNDER ITS OWN ID with
// the caller as owner. Ownership implicitly grants every permission of the
// resource's type (read live — no refresh needed), so this one call is the
// entire authz side of a create. Idempotent: an already-registered resource
// counts as success.
func (a *Authz) Register(ctx context.Context, resourceType, resourceID string, opts ...RegisterOption) error {
	res := Resource{ID: resourceID, Type: resourceType}
	if c, ok := authn.ClaimsFromCtx(ctx); ok {
		res.RealmID = c.RealmID
		res.OwnerAccountID = c.Subject
	}
	for _, opt := range opts {
		opt(&res)
	}
	if res.OwnerAccountID == "" || res.RealmID == "" {
		return apierrors.Unauthorized("no authenticated account")
	}
	if _, err := a.c.ResourceAPI().Create(ctx, res); err != nil {
		if _, gerr := a.c.ResourceAPI().Get(ctx, resourceID); gerr == nil {
			return nil
		}
		return err
	}
	return nil
}

// Unregister removes the resource's registration: the owner loses access
// immediately (the owner check reads the live row) and shared bindings leave
// the projection on the refresh triggered here — or on aegis's next sweep
// tick if that refresh fails, which is why its error is not surfaced.
func (a *Authz) Unregister(ctx context.Context, resourceID string) error {
	if err := a.c.ResourceAPI().Delete(ctx, resourceID); err != nil {
		return err
	}
	_ = a.c.AuthorizationAdminAPI().Refresh(ctx)
	return nil
}

// Authorize is the guard call: nil when the caller holds the permission on the
// resource, a ready-to-serve apierror otherwise (Unauthorized without an
// authenticated caller, Forbidden on deny) — return it as-is.
func (a *Authz) Authorize(ctx context.Context, permissionID, resourceID string) error {
	ok, err := a.Can(ctx, permissionID, resourceID)
	if err != nil {
		return err
	}
	if !ok {
		return apierrors.Forbidden("permission denied")
	}
	return nil
}

// Can reports whether the caller holds the permission on the resource — for
// callers that branch on the outcome rather than fail (guards use Authorize).
func (a *Authz) Can(ctx context.Context, permissionID, resourceID string) (bool, error) {
	c, ok := authn.ClaimsFromCtx(ctx)
	if !ok || c.Subject == "" {
		return false, apierrors.Unauthorized("no authenticated account")
	}
	return a.c.AuthorizerAPI().Check(ctx, Check{
		AccountID:    c.Subject,
		ResourceID:   resourceID,
		PermissionID: permissionID,
	})
}

// AccessibleIDs lists the resource ids the caller holds the permission on —
// the input for permission-scoped list queries (filter[id][in]).
func (a *Authz) AccessibleIDs(ctx context.Context, permissionID string) ([]string, error) {
	c, ok := authn.ClaimsFromCtx(ctx)
	if !ok || c.Subject == "" {
		return nil, apierrors.Unauthorized("no authenticated account")
	}
	return a.c.AuthorizerAPI().ListAccessible(ctx, ListAccessible{
		AccountID:    c.Subject,
		PermissionID: permissionID,
	})
}
