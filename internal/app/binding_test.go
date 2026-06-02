package app_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func newBindingUsecase(t *testing.T) (
	*apptest.BindingRepository,
	*apptest.AuthzResourceRepository,
	*apptest.RoleRepository,
	*apptest.AccountRepository,
	*apptest.GroupRepository,
	app.BindingUsecase,
) {
	bindings := apptest.NewBindingRepository(t)
	resources := apptest.NewAuthzResourceRepository(t)
	roles := apptest.NewRoleRepository(t)
	accounts := apptest.NewAccountRepository(t)
	sets := apptest.NewGroupRepository(t)
	uc := app.NewBindingUsecase(bindings, resources, roles, accounts, sets, app.NoopAuditor{})
	return bindings, resources, roles, accounts, sets, uc
}

// recordingAuditor captures the last audit event for assertions.
type recordingAuditor struct {
	action       string
	resourceType string
	resourceID   string
	changes      map[string]any
	calls        int
}

func (a *recordingAuditor) Record(_ context.Context, action, resourceType, resourceID string, changes map[string]any) {
	a.action, a.resourceType, a.resourceID, a.changes = action, resourceType, resourceID, changes
	a.calls++
}

func TestBindingCreate_EmitsAuditEvent(t *testing.T) {
	bindings := apptest.NewBindingRepository(t)
	resources := apptest.NewAuthzResourceRepository(t)
	roles := apptest.NewRoleRepository(t)
	accounts := apptest.NewAccountRepository(t)
	sets := apptest.NewGroupRepository(t)
	auditor := &recordingAuditor{}
	uc := app.NewBindingUsecase(bindings, resources, roles, accounts, sets, auditor)

	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("res-1"),
			internaltest.WithAuthzResourceRealmID("r"), internaltest.WithAuthzResourceResourceType("doc")), nil)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1"), internaltest.WithAccountRealmID("r")), nil)
	bindings.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewBinding(internaltest.WithBindingID("bind-1")), nil)

	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1"),
		internaltest.WithBindingSubjectType(domain.SubjectTypeAccount), internaltest.WithBindingSubjectID("acct-1")))
	require.NoError(t, err)
	assert.Equal(t, 1, auditor.calls)
	assert.Equal(t, "binding.grant", auditor.action)
	assert.Equal(t, "bind-1", auditor.resourceID)
}

func TestBindingCreate_AccountSubject(t *testing.T) {
	bindings, resources, roles, accounts, _, uc := newBindingUsecase(t)
	want := internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1"),
		internaltest.WithBindingSubjectType(domain.SubjectTypeAccount), internaltest.WithBindingSubjectID("acct-1"))
	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("res-1"),
			internaltest.WithAuthzResourceRealmID("r"), internaltest.WithAuthzResourceResourceType("doc")), nil)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1"), internaltest.WithAccountRealmID("r")), nil)
	bindings.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchBinding(want))).
		Return(internaltest.NewBinding(internaltest.WithBindingID("bind-1")), nil)

	got, err := uc.Create(context.Background(), want)
	require.NoError(t, err)
	assert.Equal(t, "bind-1", got.ID())
}

func TestBindingCreate_GroupSubject(t *testing.T) {
	bindings, resources, roles, _, sets, uc := newBindingUsecase(t)
	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("res-1"),
			internaltest.WithAuthzResourceRealmID("r"), internaltest.WithAuthzResourceResourceType("doc")), nil)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil)
	sets.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewGroup(internaltest.WithGroupID("set-1"), internaltest.WithGroupRealmID("r")), nil)
	bindings.EXPECT().Create(mock.Anything, mock.Anything).
		Return(internaltest.NewBinding(internaltest.WithBindingID("bind-1")), nil)

	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1"),
		internaltest.WithBindingSubjectType(domain.SubjectTypeGroup), internaltest.WithBindingSubjectID("set-1")))
	require.NoError(t, err)
}

func TestBindingCreate_RejectsResourceTypeMismatch(t *testing.T) {
	// A "doc" role bound on a "workspace" resource would let a doc-grant apply
	// to a workspace — the catastrophic leak this invariant blocks.
	_, resources, roles, _, _, uc := newBindingUsecase(t)
	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("res-1"),
			internaltest.WithAuthzResourceRealmID("r"), internaltest.WithAuthzResourceResourceType("workspace")), nil)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil)

	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1"),
		internaltest.WithBindingSubjectType(domain.SubjectTypeAccount), internaltest.WithBindingSubjectID("acct-1")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestBindingCreate_RejectsCrossRealmRole(t *testing.T) {
	_, resources, roles, _, _, uc := newBindingUsecase(t)
	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("res-1"),
			internaltest.WithAuthzResourceRealmID("r"), internaltest.WithAuthzResourceResourceType("doc")), nil)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("other"), internaltest.WithRoleResourceType("doc")), nil)

	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestBindingCreate_RejectsCrossRealmSubject(t *testing.T) {
	_, resources, roles, accounts, _, uc := newBindingUsecase(t)
	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("res-1"),
			internaltest.WithAuthzResourceRealmID("r"), internaltest.WithAuthzResourceResourceType("doc")), nil)
	roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewRole(internaltest.WithRoleID("role-1"),
			internaltest.WithRoleRealmID("r"), internaltest.WithRoleResourceType("doc")), nil)
	accounts.EXPECT().Get(mock.Anything, mock.Anything).
		Return(internaltest.NewAccount(internaltest.WithAccountID("acct-1"), internaltest.WithAccountRealmID("other")), nil)

	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingResourceID("res-1"), internaltest.WithBindingRoleID("role-1"),
		internaltest.WithBindingSubjectType(domain.SubjectTypeAccount), internaltest.WithBindingSubjectID("acct-1")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestBindingCreate_RejectsUnknownResource(t *testing.T) {
	_, resources, _, _, _, uc := newBindingUsecase(t)
	resources.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("resource", "ghost"))

	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingResourceID("ghost")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestBindingCreate_Validates(t *testing.T) {
	_, _, _, _, _, uc := newBindingUsecase(t)
	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingSubjectType("BOGUS")))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestBindingCreate_RejectsPastExpiry(t *testing.T) {
	_, _, _, _, _, uc := newBindingUsecase(t)
	past := time.Now().Add(-time.Hour)
	_, err := uc.Create(context.Background(), internaltest.NewBinding(
		internaltest.WithBindingExpiresAt(&past)))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
