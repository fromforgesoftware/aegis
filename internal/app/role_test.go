package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newRoleUsecase(t *testing.T) (
	*apptest.RoleRepository,
	*apptest.PermissionRepository,
	*apptest.RolePermissionRepository,
	app.RoleUsecase,
) {
	roles := apptest.NewRoleRepository(t)
	permissions := apptest.NewPermissionRepository(t)
	links := apptest.NewRolePermissionRepository(t)
	compositions := apptest.NewRoleCompositionRepository(t)
	uc := app.NewRoleUsecase(roles, permissions, links, compositions, persistencetest.NewTransactioner())
	return roles, permissions, links, uc
}

func TestRoleCreate_AttachesPermissions(t *testing.T) {
	roles, permissions, links, uc := newRoleUsecase(t)
	want := internaltest.NewRole(internaltest.WithRoleRealmID("r"), internaltest.WithRoleName("editor"), internaltest.WithRoleResourceType("doc"))
	roles.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchRole(want))).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"), internaltest.WithRoleRealmID("r"),
			internaltest.WithRoleName("editor"), internaltest.WithRoleResourceType("doc")), nil)
	permissions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.read", "doc", "read"), nil).Once()
	permissions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.write", "doc", "write"), nil).Once()
	links.EXPECT().DeleteByRole(mock.Anything, "role-1").Return(nil)
	links.EXPECT().CreateMany(mock.Anything, "role-1", []string{"doc.read", "doc.write"}).Return(nil)

	got, err := uc.Create(context.Background(),
		internaltest.NewRole(internaltest.WithRoleRealmID("r"), internaltest.WithRoleName("editor"), internaltest.WithRoleResourceType("doc")),
		[]string{"doc.read", "doc.write"})
	require.NoError(t, err)
	assert.Equal(t, "role-1", got.ID())
}

func TestRoleCreate_RejectsCrossResourceTypePermission(t *testing.T) {
	// Attaching a "trade.place" permission to a role with resource_type="doc"
	// would let a "doc.editor" binding grant trade-place — catastrophic.
	roles, permissions, _, uc := newRoleUsecase(t)
	roles.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"), internaltest.WithRoleResourceType("doc")), nil)
	permissions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("trade.place", "trade", "place"), nil)

	_, err := uc.Create(context.Background(),
		internaltest.NewRole(internaltest.WithRoleRealmID("r"), internaltest.WithRoleName("bad"), internaltest.WithRoleResourceType("doc")),
		[]string{"trade.place"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRoleCreate_RejectsUnknownPermission(t *testing.T) {
	roles, permissions, _, uc := newRoleUsecase(t)
	roles.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"), internaltest.WithRoleResourceType("doc")), nil)
	permissions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("permission", "doc.ghost"))

	_, err := uc.Create(context.Background(),
		internaltest.NewRole(internaltest.WithRoleRealmID("r"), internaltest.WithRoleName("bad"), internaltest.WithRoleResourceType("doc")),
		[]string{"doc.ghost"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRoleCreate_EmptyPermissionsIsAllowed(t *testing.T) {
	// A role with no permissions is a valid placeholder admins fill in later.
	roles, _, links, uc := newRoleUsecase(t)
	roles.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"), internaltest.WithRoleResourceType("doc")), nil)
	links.EXPECT().DeleteByRole(mock.Anything, "role-1").Return(nil)
	links.EXPECT().CreateMany(mock.Anything, "role-1", []string(nil)).Return(nil)

	_, err := uc.Create(context.Background(),
		internaltest.NewRole(internaltest.WithRoleRealmID("r"), internaltest.WithRoleName("empty"), internaltest.WithRoleResourceType("doc")),
		nil)
	require.NoError(t, err)
}

func TestRoleCreate_Validates(t *testing.T) {
	_, _, _, uc := newRoleUsecase(t)
	_, err := uc.Create(context.Background(),
		internaltest.NewRole(internaltest.WithRoleRealmID(""), internaltest.WithRoleName("x"), internaltest.WithRoleResourceType("doc")),
		nil)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRoleSetPermissions_OverwritesAtomically(t *testing.T) {
	roles, permissions, links, uc := newRoleUsecase(t)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"), internaltest.WithRoleResourceType("doc")), nil)
	permissions.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.read", "doc", "read"), nil)
	links.EXPECT().DeleteByRole(mock.Anything, "role-1").Return(nil)
	links.EXPECT().CreateMany(mock.Anything, "role-1", []string{"doc.read"}).Return(nil)

	require.NoError(t, uc.SetPermissions(context.Background(), "role-1", []string{"doc.read"}))
}

func newRoleUsecaseWithComposition(t *testing.T) (
	*apptest.RoleRepository,
	*apptest.RoleCompositionRepository,
	app.RoleUsecase,
) {
	roles := apptest.NewRoleRepository(t)
	permissions := apptest.NewPermissionRepository(t)
	links := apptest.NewRolePermissionRepository(t)
	compositions := apptest.NewRoleCompositionRepository(t)
	uc := app.NewRoleUsecase(roles, permissions, links, compositions, persistencetest.NewTransactioner())
	return roles, compositions, uc
}

func TestRoleSetComposition_OverwritesAtomically(t *testing.T) {
	roles, comps, uc := newRoleUsecaseWithComposition(t)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("editor"), internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil).Once()
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("viewer"), internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil).Once()
	comps.EXPECT().DeleteByRole(mock.Anything, "editor").Return(nil)
	comps.EXPECT().CreateMany(mock.Anything, "editor", []domain.RoleComponent{
		{ComponentRoleID: "viewer", Operator: domain.CompositionUnion, Ordinal: 0},
	}).Return(nil)

	require.NoError(t, uc.SetComposition(context.Background(), "editor", []domain.RoleComponent{
		{ComponentRoleID: "viewer", Operator: domain.CompositionUnion, Ordinal: 0},
	}))
}

func TestRoleSetComposition_RejectsSelfComposition(t *testing.T) {
	roles, _, uc := newRoleUsecaseWithComposition(t)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("editor"), internaltest.WithRoleResourceType("doc")), nil)

	err := uc.SetComposition(context.Background(), "editor", []domain.RoleComponent{
		{ComponentRoleID: "editor", Operator: domain.CompositionUnion},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRoleSetComposition_RejectsCrossResourceTypeComponent(t *testing.T) {
	// A composite doc role can't fold a workspace role — they grant different
	// resource types, so the composed set would be incoherent.
	roles, _, uc := newRoleUsecaseWithComposition(t)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("editor"), internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil).Once()
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("ws"), internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("workspace")), nil).Once()

	err := uc.SetComposition(context.Background(), "editor", []domain.RoleComponent{
		{ComponentRoleID: "ws", Operator: domain.CompositionUnion},
	})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestRoleListPermissions(t *testing.T) {
	roles, _, links, uc := newRoleUsecase(t)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1")), nil)
	links.EXPECT().ListPermissionIDs(mock.Anything, "role-1").
		Return([]string{"doc.read", "doc.write"}, nil)

	got, err := uc.ListPermissions(context.Background(), "role-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"doc.read", "doc.write"}, got)
}
