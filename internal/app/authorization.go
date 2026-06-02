package app

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// AuthorizationProjectionRepository rebuilds the effective_authorizations
// materialised view — the flattened binding closure the Check hot path reads.
type AuthorizationProjectionRepository interface {
	Refresh(ctx context.Context) error
}

// AuthorizationReader reads the effective_authorizations projection. Each
// method is a single indexed lookup; the projection already encodes group
// membership, role → permission, and hierarchy inheritance.
type AuthorizationReader interface {
	Exists(ctx context.Context, accountID, resourceID, permissionID string) (bool, error)
	ListResourceIDs(ctx context.Context, accountID, permissionID string) ([]string, error)
	AllowedPairs(ctx context.Context, accountID string, checks []domain.PermissionCheck) ([]domain.PermissionCheck, error)
}

// VersionRepository tracks the read-after-write counters: write_version is
// advanced by triggers on every authz write, projection_version is what the
// last refresh published.
type VersionRepository interface {
	Versions(ctx context.Context) (write, projection int64, err error)
	PublishProjection(ctx context.Context, writeVersion int64) error
}

// AuthorizationUsecase is the authz hot path plus projection maintenance.
// Check / BatchCheck / ListAccessible read the projection directly; account
// lifecycle (disabled, banned) is enforced upstream at the session layer, so
// these answer the structural question "does this grant exist".
//
// Each read takes a minVersion: a caller that just wrote can pass the
// write_version it observed, and the read is rejected as stale (precondition
// failed) until the projection has caught up — read-after-write closure.
type AuthorizationUsecase interface {
	Refresh(ctx context.Context) error
	Version(ctx context.Context) (write, projection int64, err error)
	Check(ctx context.Context, accountID, resourceID, permissionID string, minVersion int64) (bool, error)
	ListAccessible(ctx context.Context, accountID, permissionID string, minVersion int64) ([]string, error)
	BatchCheck(ctx context.Context, accountID string, checks []domain.PermissionCheck, minVersion int64) ([]domain.PermissionDecision, error)
}

type authorizationUsecase struct {
	projection AuthorizationProjectionRepository
	reader     AuthorizationReader
	resolver   RoleResolver
	version    VersionRepository
}

func NewAuthorizationUsecase(
	projection AuthorizationProjectionRepository,
	reader AuthorizationReader,
	resolver RoleResolver,
	version VersionRepository,
) AuthorizationUsecase {
	return &authorizationUsecase{projection: projection, reader: reader, resolver: resolver, version: version}
}

// Refresh captures the write_version before resolving + rebuilding the
// projection, then publishes it. Capturing first is conservative: writes that
// land during the cycle keep a higher version and stay "stale" until the next
// refresh, so a read is never falsely reported fresh.
func (uc *authorizationUsecase) Refresh(ctx context.Context) error {
	writeVersion, _, err := uc.version.Versions(ctx)
	if err != nil {
		return err
	}
	if err := uc.resolver.Resolve(ctx); err != nil {
		return err
	}
	if err := uc.projection.Refresh(ctx); err != nil {
		return err
	}
	return uc.version.PublishProjection(ctx, writeVersion)
}

func (uc *authorizationUsecase) Version(ctx context.Context) (write, projection int64, err error) {
	return uc.version.Versions(ctx)
}

func (uc *authorizationUsecase) Check(ctx context.Context, accountID, resourceID, permissionID string, minVersion int64) (bool, error) {
	if accountID == "" || resourceID == "" || permissionID == "" {
		return false, apierrors.InvalidArgument("account_id, resource_id and permission_id are required")
	}
	if err := uc.ensureFresh(ctx, minVersion); err != nil {
		return false, err
	}
	return uc.reader.Exists(ctx, accountID, resourceID, permissionID)
}

func (uc *authorizationUsecase) ListAccessible(ctx context.Context, accountID, permissionID string, minVersion int64) ([]string, error) {
	if accountID == "" || permissionID == "" {
		return nil, apierrors.InvalidArgument("account_id and permission_id are required")
	}
	if err := uc.ensureFresh(ctx, minVersion); err != nil {
		return nil, err
	}
	return uc.reader.ListResourceIDs(ctx, accountID, permissionID)
}

func (uc *authorizationUsecase) BatchCheck(ctx context.Context, accountID string, checks []domain.PermissionCheck, minVersion int64) ([]domain.PermissionDecision, error) {
	if accountID == "" {
		return nil, apierrors.InvalidArgument("account_id is required")
	}
	for _, c := range checks {
		if c.ResourceID == "" || c.PermissionID == "" {
			return nil, apierrors.InvalidArgument("each check needs a resource_id and permission_id")
		}
	}
	if err := uc.ensureFresh(ctx, minVersion); err != nil {
		return nil, err
	}
	allowed, err := uc.reader.AllowedPairs(ctx, accountID, checks)
	if err != nil {
		return nil, err
	}
	granted := make(map[domain.PermissionCheck]bool, len(allowed))
	for _, c := range allowed {
		granted[c] = true
	}
	decisions := make([]domain.PermissionDecision, len(checks))
	for i, c := range checks {
		decisions[i] = domain.PermissionDecision{
			ResourceID:   c.ResourceID,
			PermissionID: c.PermissionID,
			Allowed:      granted[c],
		}
	}
	return decisions, nil
}

// ensureFresh rejects a read whose minVersion the projection hasn't reached
// yet, so a caller never observes a pre-write answer for a write it knows
// landed.
func (uc *authorizationUsecase) ensureFresh(ctx context.Context, minVersion int64) error {
	if minVersion <= 0 {
		return nil
	}
	_, projection, err := uc.version.Versions(ctx)
	if err != nil {
		return err
	}
	if projection < minVersion {
		return apierrors.New(apierrors.CodePreconditionFailed,
			apierrors.WithMessage("authorization projection is stale; refresh and retry"))
	}
	return nil
}
