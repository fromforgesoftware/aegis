package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

const (
	permOrgRead          = "organization.read"
	permOrgUpdate        = "organization.update"
	permOrgDelete        = "organization.delete"
	permOrgMembersManage = "organization.manageMembers"

	orgRoleOwner  = "Owner"
	orgRoleAdmin  = "Admin"
	orgRoleMember = "Member"
)

// OrganizationRepository persists organizations via kit generics.
type OrganizationRepository interface {
	repository.Creator[domain.Organization]
	repository.Getter[domain.Organization]
	repository.Lister[domain.Organization]
	repository.Patcher[domain.Organization]
	repository.Deleter
	GetBySlug(ctx context.Context, realmID, slug string) (domain.Organization, error)
}

// AccountActiveOrgRepository stores the per-account active organization that
// freshly-issued access tokens carry.
type AccountActiveOrgRepository interface {
	Get(ctx context.Context, accountID string) (orgID, orgRole string, found bool, err error)
	Set(ctx context.Context, accountID, orgID, orgRole string) error
}

// OrganizationUsecase is the tenant surface. Create atomically registers the
// org's anchor authz resource, seeds the realm's org RBAC, and grants the
// owner binding; membership is expressed as bindings on that anchor.
type OrganizationUsecase interface {
	repository.Getter[domain.Organization]
	repository.Lister[domain.Organization]
	repository.Patcher[domain.Organization]
	repository.Deleter
	Create(ctx context.Context, org domain.Organization) (domain.Organization, error)
	ListForAccount(ctx context.Context, accountID string) ([]domain.Organization, error)
	// ActiveOrg resolves the org a token should carry: the account's stored
	// active org (if still a member), else its sole membership, else empty.
	ActiveOrg(ctx context.Context, accountID string) (orgID, orgRole string, err error)
	// Activate sets the account's active org after verifying membership; the
	// next issued/refreshed token carries it.
	Activate(ctx context.Context, accountID, orgID string) error
	AddMember(ctx context.Context, orgID, accountID, roleName string) error
	RemoveMember(ctx context.Context, orgID, accountID string) error
	ListMembers(ctx context.Context, orgID string) ([]domain.Binding, error)
}

type organizationUsecase struct {
	usecase.Getter[domain.Organization]
	usecase.Lister[domain.Organization]
	repository.Patcher[domain.Organization]
	repository.Deleter

	orgs        OrganizationRepository
	resources   AuthzResourceUsecase
	bindings    BindingUsecase
	roles       RoleUsecase
	permissions PermissionUsecase
	authz       AuthorizationUsecase
	activeOrg   AccountActiveOrgRepository
	auditor     Auditor
	tx          persistence.Transactioner
}

func NewOrganizationUsecase(
	orgs OrganizationRepository,
	resources AuthzResourceUsecase,
	bindings BindingUsecase,
	roles RoleUsecase,
	permissions PermissionUsecase,
	authz AuthorizationUsecase,
	activeOrg AccountActiveOrgRepository,
	auditor Auditor,
	tx persistence.Transactioner,
) OrganizationUsecase {
	return &organizationUsecase{
		Getter:      usecase.NewGetter(orgs, domain.ResourceTypeOrganization),
		Lister:      usecase.NewLister(orgs),
		Patcher:     orgs,
		Deleter:     usecase.NewDeleter(orgs),
		orgs:        orgs,
		resources:   resources,
		bindings:    bindings,
		roles:       roles,
		permissions: permissions,
		authz:       authz,
		activeOrg:   activeOrg,
		auditor:     auditor,
		tx:          tx,
	}
}

func (uc *organizationUsecase) Create(ctx context.Context, org domain.Organization) (domain.Organization, error) {
	if err := validateOrganization(org); err != nil {
		return nil, err
	}
	realmID := org.Realm().ID()
	var ownerID string
	if o := org.Owner(); o != nil {
		ownerID = o.ID()
	}

	var out domain.Organization
	err := uc.tx.Exec(ctx, func(ctx context.Context) error {
		anchorOpts := []domain.AuthzResourceOption{}
		if ownerID != "" {
			anchorOpts = append(anchorOpts, domain.WithAuthzResourceOwnerAccountID(ownerID))
		}
		anchor, err := uc.resources.Create(ctx, domain.NewAuthzResource(realmID, domain.AuthzResourceTypeOrganization, anchorOpts...))
		if err != nil {
			return err
		}
		created, err := uc.orgs.Create(ctx, domain.NewOrganization(realmID, org.Name(), org.Slug(),
			domain.WithOrganizationResourceID(anchor.ID()),
			domain.WithOrganizationOwnerID(ownerID),
			domain.WithOrganizationStatus(org.Status()),
			domain.WithOrganizationSettings(org.Settings()),
		))
		if err != nil {
			return err
		}
		ownerRoleID, err := uc.ensureOrgRBAC(ctx, realmID)
		if err != nil {
			return err
		}
		if ownerID != "" {
			if _, err := uc.bindings.Create(ctx, domain.NewBinding(anchor.ID(), ownerRoleID, domain.SubjectTypeAccount, ownerID)); err != nil {
				return err
			}
		}
		out = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := uc.authz.Refresh(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete soft-deletes the org, revokes the bindings on its anchor resource,
// soft-deletes the anchor, emits organization.deleted, and refreshes the
// projection. Consumers cascade their own org_id-keyed rows off the event.
func (uc *organizationUsecase) Delete(ctx context.Context, delType repository.DeleteType, opts ...search.Option) error {
	org, err := uc.orgs.Get(ctx, opts...)
	if err != nil {
		return err
	}
	var anchorID string
	if a := org.AnchorResource(); a != nil {
		anchorID = a.ID()
	}
	err = uc.tx.Exec(ctx, func(ctx context.Context) error {
		if anchorID != "" {
			binds, err := uc.bindings.List(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ResourceID, anchorID)))
			if err != nil {
				return err
			}
			for _, b := range binds.Results() {
				if err := uc.bindings.Delete(ctx, delType, byID(b.ID())); err != nil {
					return err
				}
			}
			if err := uc.resources.Delete(ctx, delType, byID(anchorID)); err != nil {
				return err
			}
		}
		return uc.orgs.Delete(ctx, delType, byID(org.ID()))
	})
	if err != nil {
		return err
	}
	uc.auditor.Record(ctx, "organization.deleted", string(domain.ResourceTypeOrganization), org.ID(), nil)
	return uc.authz.Refresh(ctx)
}

func (uc *organizationUsecase) ListForAccount(ctx context.Context, accountID string) ([]domain.Organization, error) {
	if accountID == "" {
		return nil, apierrors.InvalidArgument("account_id is required")
	}
	resourceIDs, err := uc.authz.ListAccessible(ctx, accountID, permOrgRead, 0)
	if err != nil {
		return nil, err
	}
	if len(resourceIDs) == 0 {
		return []domain.Organization{}, nil
	}
	res, err := uc.orgs.List(ctx, search.WithQueryOpts(query.FilterBy(filter.OpIn, fields.ResourceID, resourceIDs)))
	if err != nil {
		return nil, err
	}
	return res.Results(), nil
}

func (uc *organizationUsecase) ActiveOrg(ctx context.Context, accountID string) (string, string, error) {
	stored, storedRole, found, err := uc.activeOrg.Get(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if found && stored != "" {
		role, member, err := uc.memberRole(ctx, stored, accountID)
		if err != nil {
			return "", "", err
		}
		if member {
			if role != "" {
				storedRole = role
			}
			return stored, storedRole, nil
		}
	}
	orgs, err := uc.ListForAccount(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if len(orgs) != 1 {
		return "", "", nil
	}
	org := orgs[0]
	role, err := uc.memberRoleByOrg(ctx, org, accountID)
	if err != nil {
		return "", "", err
	}
	return org.ID(), role, nil
}

func (uc *organizationUsecase) Activate(ctx context.Context, accountID, orgID string) error {
	role, member, err := uc.memberRole(ctx, orgID, accountID)
	if err != nil {
		return err
	}
	if !member {
		return apierrors.Forbidden("not a member of this organization")
	}
	return uc.activeOrg.Set(ctx, accountID, orgID, role)
}

// memberRole resolves the account's top role on orgID and whether it is a member.
func (uc *organizationUsecase) memberRole(ctx context.Context, orgID, accountID string) (string, bool, error) {
	org, err := uc.orgs.Get(ctx, byID(orgID))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	role, err := uc.memberRoleByOrg(ctx, org, accountID)
	if err != nil {
		return "", false, err
	}
	return role, role != "", nil
}

// memberRoleByOrg returns the account's highest-ranked org role name on org, or "".
func (uc *organizationUsecase) memberRoleByOrg(ctx context.Context, org domain.Organization, accountID string) (string, error) {
	anchor := org.AnchorResource()
	if anchor == nil {
		return "", nil
	}
	found, err := uc.bindings.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.ResourceID, anchor.ID()),
		query.FilterBy(filter.OpEq, fields.SubjectID, accountID),
	))
	if err != nil {
		return "", err
	}
	rank := map[string]int{orgRoleMember: 1, orgRoleAdmin: 2, orgRoleOwner: 3}
	best, bestRank := "", 0
	for _, b := range found.Results() {
		role, err := uc.roles.Get(ctx, byID(b.RoleID()))
		if err != nil {
			continue
		}
		if r := rank[role.Name()]; r > bestRank {
			bestRank, best = r, role.Name()
		}
	}
	return best, nil
}

func (uc *organizationUsecase) AddMember(ctx context.Context, orgID, accountID, roleName string) error {
	anchorID, realmID, err := uc.anchor(ctx, orgID)
	if err != nil {
		return err
	}
	roleID, err := uc.resolveOrgRole(ctx, realmID, roleName)
	if err != nil {
		return err
	}
	if _, err := uc.bindings.Create(ctx, domain.NewBinding(anchorID, roleID, domain.SubjectTypeAccount, accountID)); err != nil {
		return err
	}
	return uc.authz.Refresh(ctx)
}

func (uc *organizationUsecase) RemoveMember(ctx context.Context, orgID, accountID string) error {
	anchorID, _, err := uc.anchor(ctx, orgID)
	if err != nil {
		return err
	}
	found, err := uc.bindings.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.ResourceID, anchorID),
		query.FilterBy(filter.OpEq, fields.SubjectID, accountID),
	))
	if err != nil {
		return err
	}
	for _, b := range found.Results() {
		if err := uc.bindings.Delete(ctx, repository.DeleteTypeHard, byID(b.ID())); err != nil {
			return err
		}
	}
	return uc.authz.Refresh(ctx)
}

func (uc *organizationUsecase) ListMembers(ctx context.Context, orgID string) ([]domain.Binding, error) {
	anchorID, _, err := uc.anchor(ctx, orgID)
	if err != nil {
		return nil, err
	}
	found, err := uc.bindings.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.ResourceID, anchorID),
		query.FilterBy(filter.OpEq, fields.SubjectType, string(domain.SubjectTypeAccount)),
	))
	if err != nil {
		return nil, err
	}
	return found.Results(), nil
}

// anchor resolves an org's anchor resource id and realm.
func (uc *organizationUsecase) anchor(ctx context.Context, orgID string) (anchorID, realmID string, err error) {
	org, err := uc.orgs.Get(ctx, byID(orgID))
	if err != nil {
		return "", "", err
	}
	res := org.AnchorResource()
	if res == nil {
		return "", "", apierrors.InvalidArgument("organization has no anchor resource")
	}
	return res.ID(), org.Realm().ID(), nil
}

// ensureOrgRBAC seeds the realm's organization permissions + Owner/Admin/Member
// roles idempotently and returns the Owner role id.
func (uc *organizationUsecase) ensureOrgRBAC(ctx context.Context, realmID string) (string, error) {
	perms := []struct{ id, verb string }{
		{permOrgRead, "read"},
		{permOrgUpdate, "update"},
		{permOrgDelete, "delete"},
		{permOrgMembersManage, "manageMembers"},
	}
	for _, p := range perms {
		if err := uc.ensurePermission(ctx, p.id, p.verb); err != nil {
			return "", err
		}
	}
	memberID, err := uc.ensureRole(ctx, realmID, orgRoleMember, []string{permOrgRead})
	if err != nil {
		return "", err
	}
	adminID, err := uc.ensureRole(ctx, realmID, orgRoleAdmin, []string{permOrgRead, permOrgUpdate, permOrgMembersManage})
	if err != nil {
		return "", err
	}
	ownerID, err := uc.ensureRole(ctx, realmID, orgRoleOwner, []string{permOrgRead, permOrgUpdate, permOrgDelete, permOrgMembersManage})
	if err != nil {
		return "", err
	}
	if err := uc.roles.SetComposition(ctx, adminID, []domain.RoleComponent{{ComponentRoleID: memberID, Operator: domain.CompositionUnion, Ordinal: 0}}); err != nil {
		return "", err
	}
	if err := uc.roles.SetComposition(ctx, ownerID, []domain.RoleComponent{{ComponentRoleID: adminID, Operator: domain.CompositionUnion, Ordinal: 0}}); err != nil {
		return "", err
	}
	return ownerID, nil
}

func (uc *organizationUsecase) ensurePermission(ctx context.Context, id, verb string) error {
	if _, err := uc.permissions.Get(ctx, byID(id)); err == nil {
		return nil
	} else if !apierrors.Is(err, apierrors.CodeNotFound) {
		return err
	}
	_, err := uc.permissions.Create(ctx, domain.NewPermission(id, domain.AuthzResourceTypeOrganization, verb))
	return err
}

func (uc *organizationUsecase) ensureRole(ctx context.Context, realmID, name string, permIDs []string) (string, error) {
	if id, err := uc.findOrgRole(ctx, realmID, name); err != nil {
		return "", err
	} else if id != "" {
		return id, nil
	}
	role, err := uc.roles.Create(ctx,
		domain.NewRole(realmID, name, domain.AuthzResourceTypeOrganization, domain.WithRoleKind(domain.RoleKindSystem)),
		permIDs)
	if err != nil {
		return "", err
	}
	return role.ID(), nil
}

func (uc *organizationUsecase) resolveOrgRole(ctx context.Context, realmID, name string) (string, error) {
	id, err := uc.findOrgRole(ctx, realmID, name)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", apierrors.InvalidArgument("unknown organization role")
	}
	return id, nil
}

func (uc *organizationUsecase) findOrgRole(ctx context.Context, realmID, name string) (string, error) {
	found, err := uc.roles.List(ctx, search.WithQueryOpts(
		query.FilterBy(filter.OpEq, fields.RealmID, realmID),
		query.FilterBy(filter.OpEq, fields.Name, name),
		query.FilterBy(filter.OpEq, fields.ResourceType, domain.AuthzResourceTypeOrganization),
	))
	if err != nil {
		return "", err
	}
	if len(found.Results()) == 0 {
		return "", nil
	}
	return found.Results()[0].ID(), nil
}

func validateOrganization(o domain.Organization) error {
	if o.Realm().ID() == "" {
		return apierrors.InvalidArgument("realm is required")
	}
	if o.Name() == "" {
		return apierrors.InvalidArgument("name is required")
	}
	if o.Slug() == "" {
		return apierrors.InvalidArgument("slug is required")
	}
	if !o.Status().Valid() {
		return apierrors.InvalidArgument("invalid status")
	}
	return nil
}
