package app_test

import (
	"context"
	"errors"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func newAuthorizationUsecase(t *testing.T) (
	*apptest.AuthorizationProjectionRepository,
	*apptest.AuthorizationReader,
	*apptest.RoleResolver,
	*apptest.VersionRepository,
	app.AuthorizationUsecase,
) {
	projection := apptest.NewAuthorizationProjectionRepository(t)
	reader := apptest.NewAuthorizationReader(t)
	resolver := apptest.NewRoleResolver(t)
	version := apptest.NewVersionRepository(t)
	return projection, reader, resolver, version, app.NewAuthorizationUsecase(projection, reader, resolver, version)
}

func TestAuthorizationRefresh_CapturesResolvesRefreshesPublishes(t *testing.T) {
	projection, _, resolver, version, uc := newAuthorizationUsecase(t)
	version.EXPECT().Versions(mock.Anything).Return(int64(7), int64(3), nil)
	resolver.EXPECT().Resolve(mock.Anything).Return(nil)
	projection.EXPECT().Refresh(mock.Anything).Return(nil)
	// Publishes the write_version captured before the cycle, not after.
	version.EXPECT().PublishProjection(mock.Anything, int64(7)).Return(nil)
	require.NoError(t, uc.Refresh(context.Background()))
}

func TestAuthorizationRefresh_SkipsRefreshWhenResolveFails(t *testing.T) {
	_, _, resolver, version, uc := newAuthorizationUsecase(t)
	version.EXPECT().Versions(mock.Anything).Return(int64(1), int64(1), nil)
	resolver.EXPECT().Resolve(mock.Anything).Return(errors.New("boom"))
	assert.Error(t, uc.Refresh(context.Background()))
}

func TestAuthorizationCheck_StaleWhenProjectionBehindMinVersion(t *testing.T) {
	_, _, _, version, uc := newAuthorizationUsecase(t)
	version.EXPECT().Versions(mock.Anything).Return(int64(9), int64(4), nil)

	_, err := uc.Check(context.Background(), "acct-1", "res-1", "doc.read", 5)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodePreconditionFailed))
}

func TestAuthorizationCheck_FreshWhenProjectionAtMinVersion(t *testing.T) {
	_, reader, _, version, uc := newAuthorizationUsecase(t)
	version.EXPECT().Versions(mock.Anything).Return(int64(9), int64(5), nil)
	reader.EXPECT().Exists(mock.Anything, "acct-1", "res-1", "doc.read").Return(true, nil)

	allowed, err := uc.Check(context.Background(), "acct-1", "res-1", "doc.read", 5)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestAuthorizationVersion(t *testing.T) {
	_, _, _, version, uc := newAuthorizationUsecase(t)
	version.EXPECT().Versions(mock.Anything).Return(int64(9), int64(5), nil)

	write, projection, err := uc.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(9), write)
	assert.Equal(t, int64(5), projection)
}

func TestAuthorizationCheck_Allowed(t *testing.T) {
	_, reader, _, _, uc := newAuthorizationUsecase(t)
	reader.EXPECT().Exists(mock.Anything, "acct-1", "res-1", "doc.read").Return(true, nil)

	allowed, err := uc.Check(context.Background(), "acct-1", "res-1", "doc.read", 0)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestAuthorizationCheck_Validates(t *testing.T) {
	_, _, _, _, uc := newAuthorizationUsecase(t)
	_, err := uc.Check(context.Background(), "acct-1", "", "doc.read", 0)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestAuthorizationListAccessible(t *testing.T) {
	_, reader, _, _, uc := newAuthorizationUsecase(t)
	reader.EXPECT().ListResourceIDs(mock.Anything, "acct-1", "doc.read").
		Return([]string{"res-1", "res-2"}, nil)

	got, err := uc.ListAccessible(context.Background(), "acct-1", "doc.read", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"res-1", "res-2"}, got)
}

func TestAuthorizationListAccessible_Validates(t *testing.T) {
	_, _, _, _, uc := newAuthorizationUsecase(t)
	_, err := uc.ListAccessible(context.Background(), "", "doc.read", 0)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestAuthorizationBatchCheck_MapsDecisions(t *testing.T) {
	_, reader, _, _, uc := newAuthorizationUsecase(t)
	checks := []domain.PermissionCheck{
		{ResourceID: "res-1", PermissionID: "doc.read"},
		{ResourceID: "res-1", PermissionID: "doc.write"},
		{ResourceID: "res-2", PermissionID: "doc.read"},
	}
	// Only the first and third are granted; the write is denied.
	reader.EXPECT().AllowedPairs(mock.Anything, "acct-1", checks).Return([]domain.PermissionCheck{
		{ResourceID: "res-1", PermissionID: "doc.read"},
		{ResourceID: "res-2", PermissionID: "doc.read"},
	}, nil)

	got, err := uc.BatchCheck(context.Background(), "acct-1", checks, 0)
	require.NoError(t, err)
	assert.Equal(t, []domain.PermissionDecision{
		{ResourceID: "res-1", PermissionID: "doc.read", Allowed: true},
		{ResourceID: "res-1", PermissionID: "doc.write", Allowed: false},
		{ResourceID: "res-2", PermissionID: "doc.read", Allowed: true},
	}, got)
}

func TestAuthorizationBatchCheck_Validates(t *testing.T) {
	_, _, _, _, uc := newAuthorizationUsecase(t)
	_, err := uc.BatchCheck(context.Background(), "acct-1", []domain.PermissionCheck{
		{ResourceID: "", PermissionID: "doc.read"},
	}, 0)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
