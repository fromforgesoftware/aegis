package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// GroupRepository persists groups via kit generics.
type GroupRepository interface {
	repository.Creator[domain.Group]
	repository.Getter[domain.Group]
	repository.Lister[domain.Group]
	repository.Patcher[domain.Group]
	repository.Deleter
}

// GroupMemberRepository persists the group↔account junction (the
// aegis.actor_set_member table). The atomic overwrite of a group's membership
// is composed at the usecase layer: DeleteByGroup + CreateMany inside one
// Transactioner.Exec.
type GroupMemberRepository interface {
	DeleteByGroup(ctx context.Context, groupID string) error
	CreateMany(ctx context.Context, groupID string, accountIDs []string) error
	ListAccountIDs(ctx context.Context, groupID string) ([]string, error)
}

// GroupUsecase is the admin surface for groups. Create persists the set
// then attaches the requested members atomically; SetMembers overwrites the
// membership, validating every account exists inside the set's realm so a
// group can't grant cross-realm access once it's bound.
type GroupUsecase interface {
	repository.Getter[domain.Group]
	repository.Lister[domain.Group]
	repository.Patcher[domain.Group]
	repository.Deleter
	Create(ctx context.Context, set domain.Group, accountIDs []string) (domain.Group, error)
	SetMembers(ctx context.Context, groupID string, accountIDs []string) error
	ListMembers(ctx context.Context, groupID string) ([]string, error)
}

type groupUsecase struct {
	usecase.Getter[domain.Group]
	usecase.Lister[domain.Group]
	repository.Patcher[domain.Group]
	repository.Deleter

	sets     GroupRepository
	members  GroupMemberRepository
	accounts AccountRepository
	tx       persistence.Transactioner
}

func NewGroupUsecase(
	sets GroupRepository,
	members GroupMemberRepository,
	accounts AccountRepository,
	tx persistence.Transactioner,
) GroupUsecase {
	return &groupUsecase{
		Getter:   usecase.NewGetter(sets, domain.ResourceTypeGroup),
		Lister:   usecase.NewLister(sets),
		Patcher:  sets,
		Deleter:  usecase.NewDeleter(sets),
		sets:     sets,
		members:  members,
		accounts: accounts,
		tx:       tx,
	}
}

func (uc *groupUsecase) Create(ctx context.Context, set domain.Group, accountIDs []string) (domain.Group, error) {
	if err := validateGroup(set); err != nil {
		return nil, err
	}

	var out domain.Group
	err := uc.tx.Exec(ctx, func(ctx context.Context) error {
		created, err := uc.sets.Create(ctx, set)
		if err != nil {
			return err
		}
		if err := uc.attachMembers(ctx, created.ID(), set.RealmID(), accountIDs); err != nil {
			return err
		}
		out = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (uc *groupUsecase) SetMembers(ctx context.Context, groupID string, accountIDs []string) error {
	set, err := uc.sets.Get(ctx, byID(groupID))
	if err != nil {
		return err
	}
	return uc.attachMembers(ctx, groupID, set.RealmID(), accountIDs)
}

func (uc *groupUsecase) ListMembers(ctx context.Context, groupID string) ([]string, error) {
	if _, err := uc.sets.Get(ctx, byID(groupID)); err != nil {
		return nil, err
	}
	return uc.members.ListAccountIDs(ctx, groupID)
}

// attachMembers verifies every account exists inside the set's realm and
// overwrites the membership atomically (DELETE-then-INSERT inside the
// Transactioner). Empty accountIDs clears the set.
func (uc *groupUsecase) attachMembers(ctx context.Context, groupID, realmID string, accountIDs []string) error {
	for _, aid := range accountIDs {
		acct, err := uc.accounts.Get(ctx, byID(aid))
		if err != nil {
			if apierrors.Is(err, apierrors.CodeNotFound) {
				return apierrors.InvalidArgument("unknown account")
			}
			return err
		}
		if acct.RealmID() != realmID {
			return apierrors.InvalidArgument("account belongs to a different realm")
		}
	}
	return uc.tx.Exec(ctx, func(ctx context.Context) error {
		if err := uc.members.DeleteByGroup(ctx, groupID); err != nil {
			return err
		}
		return uc.members.CreateMany(ctx, groupID, accountIDs)
	})
}

func validateGroup(s domain.Group) error {
	if s.RealmID() == "" {
		return apierrors.InvalidArgument("realm_id is required")
	}
	if s.Name() == "" {
		return apierrors.InvalidArgument("name is required")
	}
	return nil
}
