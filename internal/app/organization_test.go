package app_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/application/repository"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

type orgMocks struct {
	orgs      *apptest.OrganizationRepository
	resources *apptest.AuthzResourceUsecase
	bindings  *apptest.BindingUsecase
	roles     *apptest.RoleUsecase
	perms     *apptest.PermissionUsecase
	authz     *apptest.AuthorizationUsecase
	active    *apptest.AccountActiveOrgRepository
	auditor   *apptest.Auditor
}

func newOrgUsecase(t *testing.T) (orgMocks, app.OrganizationUsecase) {
	m := orgMocks{
		orgs:      apptest.NewOrganizationRepository(t),
		resources: apptest.NewAuthzResourceUsecase(t),
		bindings:  apptest.NewBindingUsecase(t),
		roles:     apptest.NewRoleUsecase(t),
		perms:     apptest.NewPermissionUsecase(t),
		authz:     apptest.NewAuthorizationUsecase(t),
		active:    apptest.NewAccountActiveOrgRepository(t),
		auditor:   apptest.NewAuditor(t),
	}
	uc := app.NewOrganizationUsecase(m.orgs, m.resources, m.bindings, m.roles, m.perms,
		m.authz, m.active, m.auditor, persistencetest.NewTransactioner())
	return m, uc
}

func orgFixture(id, resID, realm string) domain.Organization {
	return internaltest.NewOrganization(
		internaltest.WithOrganizationID(id),
		internaltest.WithOrganizationResourceID(resID),
		internaltest.WithOrganizationRealmID(realm),
	)
}

func bindingList(bs ...domain.Binding) resource.ListResponse[domain.Binding] {
	return resource.NewListResponse(bs, len(bs))
}

func TestOrgCreate_SeedsAnchorOwnerBindingAndRefreshes(t *testing.T) {
	m, uc := newOrgUsecase(t)

	m.resources.EXPECT().Create(mock.Anything, mock.Anything).
		Return(domain.NewAuthzResource("r", domain.AuthzResourceTypeOrganization, domain.WithAuthzResourceID("res-1")), nil)
	m.orgs.EXPECT().Create(mock.Anything, mock.Anything).
		Return(orgFixture("org-1", "res-1", "r"), nil)
	// org permissions already seeded → Get finds them, no Create.
	m.perms.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("organization.read", domain.AuthzResourceTypeOrganization, "read"), nil)
	// roles already seeded → List returns each tier (Member, Admin, Owner) in order.
	m.roles.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse([]domain.Role{domain.NewRole("r", "Member", domain.AuthzResourceTypeOrganization, domain.WithRoleID("role-member"))}, 1), nil).Once()
	m.roles.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse([]domain.Role{domain.NewRole("r", "Admin", domain.AuthzResourceTypeOrganization, domain.WithRoleID("role-admin"))}, 1), nil).Once()
	m.roles.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse([]domain.Role{domain.NewRole("r", "Owner", domain.AuthzResourceTypeOrganization, domain.WithRoleID("role-owner"))}, 1), nil).Once()
	m.roles.EXPECT().SetComposition(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	m.bindings.EXPECT().Create(mock.Anything, mock.MatchedBy(func(b domain.Binding) bool {
		return b.ResourceID() == "res-1" && b.RoleID() == "role-owner" &&
			b.SubjectType() == domain.SubjectTypeAccount && b.SubjectID() == "acc-1"
	})).Return(domain.NewBinding("res-1", "role-owner", domain.SubjectTypeAccount, "acc-1"), nil)
	m.authz.EXPECT().Refresh(mock.Anything).Return(nil)

	in := internaltest.NewOrganization(
		internaltest.WithOrganizationRealmID("r"),
		internaltest.WithOrganizationName("Acme"),
		internaltest.WithOrganizationSlug("acme"),
		internaltest.WithOrganizationOwnerID("acc-1"),
	)
	got, err := uc.Create(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "org-1", got.ID())
}

func TestActivate_RejectsNonMember(t *testing.T) {
	m, uc := newOrgUsecase(t)
	m.orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(orgFixture("org-1", "res-1", "r"), nil)
	m.bindings.EXPECT().List(mock.Anything, mock.Anything).Return(bindingList(), nil)

	err := uc.Activate(context.Background(), "acc-1", "org-1")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeForbidden))
}

func TestActivate_SetsActiveOrg(t *testing.T) {
	m, uc := newOrgUsecase(t)
	m.orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(orgFixture("org-1", "res-1", "r"), nil)
	m.bindings.EXPECT().List(mock.Anything, mock.Anything).
		Return(bindingList(domain.NewBinding("res-1", "role-owner", domain.SubjectTypeAccount, "acc-1")), nil)
	m.roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewRole("r", "Owner", domain.AuthzResourceTypeOrganization), nil)
	m.active.EXPECT().Set(mock.Anything, "acc-1", "org-1", "Owner").Return(nil)

	require.NoError(t, uc.Activate(context.Background(), "acc-1", "org-1"))
}

func TestActiveOrg_PrefersStoredMembership(t *testing.T) {
	m, uc := newOrgUsecase(t)
	m.active.EXPECT().Get(mock.Anything, "acc-1").Return("org-1", "Admin", true, nil)
	m.orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(orgFixture("org-1", "res-1", "r"), nil)
	m.bindings.EXPECT().List(mock.Anything, mock.Anything).
		Return(bindingList(domain.NewBinding("res-1", "role-admin", domain.SubjectTypeAccount, "acc-1")), nil)
	m.roles.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewRole("r", "Admin", domain.AuthzResourceTypeOrganization), nil)

	orgID, role, err := uc.ActiveOrg(context.Background(), "acc-1")
	require.NoError(t, err)
	assert.Equal(t, "org-1", orgID)
	assert.Equal(t, "Admin", role)
}

func TestActiveOrg_EmptyWhenAmbiguous(t *testing.T) {
	m, uc := newOrgUsecase(t)
	m.active.EXPECT().Get(mock.Anything, "acc-1").Return("", "", false, nil)
	m.authz.EXPECT().ListAccessible(mock.Anything, "acc-1", "organization.read", int64(0)).
		Return([]string{"res-1", "res-2"}, nil)
	m.orgs.EXPECT().List(mock.Anything, mock.Anything).
		Return(resource.NewListResponse([]domain.Organization{
			orgFixture("org-1", "res-1", "r"), orgFixture("org-2", "res-2", "r"),
		}, 2), nil)

	orgID, _, err := uc.ActiveOrg(context.Background(), "acc-1")
	require.NoError(t, err)
	assert.Empty(t, orgID, "ambiguous membership yields no active org")
}

func TestDelete_CascadesAndEmitsEvent(t *testing.T) {
	m, uc := newOrgUsecase(t)
	m.orgs.EXPECT().Get(mock.Anything, mock.Anything).Return(orgFixture("org-1", "res-1", "r"), nil)
	m.bindings.EXPECT().List(mock.Anything, mock.Anything).
		Return(bindingList(domain.NewBinding("res-1", "role-owner", domain.SubjectTypeAccount, "acc-1", domain.WithBindingID("bind-1"))), nil)
	m.bindings.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)
	m.resources.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)
	m.orgs.EXPECT().Delete(mock.Anything, repository.DeleteTypeSoft, mock.Anything).Return(nil)
	m.auditor.EXPECT().Record(mock.Anything, "organization.deleted", "organizations", "org-1", mock.Anything).Return()
	m.authz.EXPECT().Refresh(mock.Anything).Return(nil)

	err := uc.Delete(context.Background(), repository.DeleteTypeSoft,
		search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, "org-1")))
	require.NoError(t, err)
}
