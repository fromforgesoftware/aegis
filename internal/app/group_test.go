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
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newGroupUsecase(t *testing.T) (
	*apptest.GroupRepository,
	*apptest.GroupMemberRepository,
	*apptest.AccountRepository,
	app.GroupUsecase,
) {
	sets := apptest.NewGroupRepository(t)
	members := apptest.NewGroupMemberRepository(t)
	accounts := apptest.NewAccountRepository(t)
	uc := app.NewGroupUsecase(sets, members, accounts, persistencetest.NewTransactioner())
	return sets, members, accounts, uc
}

func TestGroupCreate_AttachesMembers(t *testing.T) {
	sets, members, accounts, uc := newGroupUsecase(t)
	want := internaltest.NewGroup(internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("editors"))
	sets.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchGroup(want))).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"),
			internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("editors")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1"), internaltest.WithAccountRealmID("r")), nil).Once()
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-2"), internaltest.WithAccountRealmID("r")), nil).Once()
	members.EXPECT().DeleteByGroup(mock.Anything, "set-1").Return(nil)
	members.EXPECT().CreateMany(mock.Anything, "set-1", []string{"acct-1", "acct-2"}).Return(nil)

	got, err := uc.Create(context.Background(),
		internaltest.NewGroup(internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("editors")),
		[]string{"acct-1", "acct-2"})
	require.NoError(t, err)
	assert.Equal(t, "set-1", got.ID())
}

func TestGroupCreate_RejectsCrossRealmMember(t *testing.T) {
	// A group in realm "r" can't enrol an account from realm "other" — once
	// the group is bound that would smuggle a foreign account into the realm.
	sets, _, accounts, uc := newGroupUsecase(t)
	sets.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"), internaltest.WithGroupRealmID("r")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1"), internaltest.WithAccountRealmID("other")), nil)

	_, err := uc.Create(context.Background(),
		internaltest.NewGroup(internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("g")),
		[]string{"acct-1"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestGroupCreate_RejectsUnknownMember(t *testing.T) {
	sets, _, accounts, uc := newGroupUsecase(t)
	sets.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"), internaltest.WithGroupRealmID("r")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("account", "ghost"))

	_, err := uc.Create(context.Background(),
		internaltest.NewGroup(internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("g")),
		[]string{"ghost"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestGroupCreate_EmptyMembersIsAllowed(t *testing.T) {
	sets, members, _, uc := newGroupUsecase(t)
	sets.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"), internaltest.WithGroupRealmID("r")), nil)
	members.EXPECT().DeleteByGroup(mock.Anything, "set-1").Return(nil)
	members.EXPECT().CreateMany(mock.Anything, "set-1", []string(nil)).Return(nil)

	_, err := uc.Create(context.Background(),
		internaltest.NewGroup(internaltest.WithGroupRealmID("r"), internaltest.WithGroupName("empty")), nil)
	require.NoError(t, err)
}

func TestGroupCreate_Validates(t *testing.T) {
	_, _, _, uc := newGroupUsecase(t)
	_, err := uc.Create(context.Background(),
		internaltest.NewGroup(internaltest.WithGroupRealmID("")), nil)
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestGroupSetMembers_OverwritesAtomically(t *testing.T) {
	sets, members, accounts, uc := newGroupUsecase(t)
	sets.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"), internaltest.WithGroupRealmID("r")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1"), internaltest.WithAccountRealmID("r")), nil)
	members.EXPECT().DeleteByGroup(mock.Anything, "set-1").Return(nil)
	members.EXPECT().CreateMany(mock.Anything, "set-1", []string{"acct-1"}).Return(nil)

	require.NoError(t, uc.SetMembers(context.Background(), "set-1", []string{"acct-1"}))
}

func TestGroupListMembers(t *testing.T) {
	sets, members, _, uc := newGroupUsecase(t)
	sets.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1")), nil)
	members.EXPECT().ListAccountIDs(mock.Anything, "set-1").Return([]string{"acct-1", "acct-2"}, nil)

	got, err := uc.ListMembers(context.Background(), "set-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"acct-1", "acct-2"}, got)
}
