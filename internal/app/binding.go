package app

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// BindingRepository persists ACL bindings via kit generics. Bindings are
// immutable — to change a grant, revoke (delete) and create a new one — so
// there's no Patcher. DeleteExpired backs the sweeper.
type BindingRepository interface {
	repository.Creator[domain.Binding]
	repository.Getter[domain.Binding]
	repository.Lister[domain.Binding]
	repository.Deleter
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

// BindingUsecase is the surface that grants access: it ties a subject
// (account or group) to a role on a resource. Create enforces the
// invariants that keep authz sound — the role's resource_type must match the
// bound resource's type, and role, resource, and subject must all live in the
// same realm — so a grant can never leak a permission across resource types
// or realms.
type BindingUsecase interface {
	repository.Getter[domain.Binding]
	repository.Lister[domain.Binding]
	repository.Deleter
	Create(ctx context.Context, b domain.Binding) (domain.Binding, error)
}

type bindingUsecase struct {
	usecase.Getter[domain.Binding]
	usecase.Lister[domain.Binding]
	repository.Deleter

	bindings  BindingRepository
	resources AuthzResourceRepository
	roles     RoleRepository
	accounts  AccountRepository
	groups    GroupRepository
	auditor   Auditor
}

func NewBindingUsecase(
	bindings BindingRepository,
	resources AuthzResourceRepository,
	roles RoleRepository,
	accounts AccountRepository,
	groups GroupRepository,
	auditor Auditor,
) BindingUsecase {
	return &bindingUsecase{
		Getter:    usecase.NewGetter(bindings, domain.ResourceTypeBinding),
		Lister:    usecase.NewLister(bindings),
		Deleter:   usecase.NewDeleter(bindings),
		bindings:  bindings,
		resources: resources,
		roles:     roles,
		accounts:  accounts,
		groups:    groups,
		auditor:   auditor,
	}
}

func (uc *bindingUsecase) Create(ctx context.Context, b domain.Binding) (domain.Binding, error) {
	if err := validateBinding(b); err != nil {
		return nil, err
	}

	res, err := uc.resources.Get(ctx, byID(b.ResourceID()))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil, apierrors.InvalidArgument("resource not found")
		}
		return nil, err
	}
	role, err := uc.roles.Get(ctx, byID(b.RoleID()))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return nil, apierrors.InvalidArgument("role not found")
		}
		return nil, err
	}
	if role.ResourceType() != res.ResourceType() {
		return nil, apierrors.InvalidArgument("role resource_type does not match the resource's type")
	}
	if role.RealmID() != res.RealmID() {
		return nil, apierrors.InvalidArgument("role belongs to a different realm")
	}
	if err := uc.validateSubject(ctx, b, res.RealmID()); err != nil {
		return nil, err
	}
	created, err := uc.bindings.Create(ctx, b)
	if err != nil {
		return nil, err
	}
	uc.auditor.Record(ctx, "binding.grant", "binding", created.ID(), map[string]any{
		"resourceId":  created.ResourceID(),
		"roleId":      created.RoleID(),
		"subjectType": string(created.SubjectType()),
		"subjectId":   created.SubjectID(),
	})
	return created, nil
}

// validateSubject confirms the binding's subject exists and shares the
// resource's realm, so a grant can't name an account or group from another
// realm.
func (uc *bindingUsecase) validateSubject(ctx context.Context, b domain.Binding, realmID string) error {
	switch b.SubjectType() {
	case domain.SubjectTypeAccount:
		acct, err := uc.accounts.Get(ctx, byID(b.SubjectID()))
		if err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return apierrors.InvalidArgument("subject account not found")
			}
			return err
		}
		if acct.RealmID() != realmID {
			return apierrors.InvalidArgument("subject account belongs to a different realm")
		}
	case domain.SubjectTypeGroup:
		set, err := uc.groups.Get(ctx, byID(b.SubjectID()))
		if err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return apierrors.InvalidArgument("subject group not found")
			}
			return err
		}
		if set.RealmID() != realmID {
			return apierrors.InvalidArgument("subject group belongs to a different realm")
		}
	}
	return nil
}

func validateBinding(b domain.Binding) error {
	if b.ResourceID() == "" {
		return apierrors.InvalidArgument("resource_id is required")
	}
	if b.RoleID() == "" {
		return apierrors.InvalidArgument("role_id is required")
	}
	if !b.SubjectType().Valid() {
		return apierrors.InvalidArgument("invalid subject_type")
	}
	if b.SubjectID() == "" {
		return apierrors.InvalidArgument("subject_id is required")
	}
	if exp := b.ExpiresAt(); exp != nil && !exp.After(time.Now()) {
		return apierrors.InvalidArgument("expires_at must be in the future")
	}
	return nil
}
