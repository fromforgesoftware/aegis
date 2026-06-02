package app

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// InvitationRepository persists invitations.
type InvitationRepository interface {
	repository.Creator[domain.Invitation]
	repository.Getter[domain.Invitation]
	repository.Lister[domain.Invitation]
	GetByTokenHash(ctx context.Context, tokenHash string) (domain.Invitation, error)
	MarkAccepted(ctx context.Context, id string, at time.Time) error
}

// InvitationUsecase is the admin surface for invitations. Create mints a token
// (only its hash is stored) and delivers it via NotificationSender; Accept
// validates the token and binds the accepting account to the invited role on
// the invited resource.
type InvitationUsecase interface {
	repository.Getter[domain.Invitation]
	repository.Lister[domain.Invitation]
	Create(ctx context.Context, inv domain.Invitation) (domain.Invitation, error)
	Accept(ctx context.Context, token, accountID string) error
}

type invitationUsecase struct {
	usecase.Getter[domain.Invitation]
	usecase.Lister[domain.Invitation]

	invitations InvitationRepository
	bindings    BindingUsecase
	notifier    NotificationSender
	now         func() time.Time
}

func NewInvitationUsecase(
	invitations InvitationRepository,
	bindings BindingUsecase,
	notifier NotificationSender,
) InvitationUsecase {
	return &invitationUsecase{
		Getter:      usecase.NewGetter(invitations, domain.ResourceTypeInvitation),
		Lister:      usecase.NewLister(invitations),
		invitations: invitations,
		bindings:    bindings,
		notifier:    notifier,
		now:         time.Now,
	}
}

func (uc *invitationUsecase) Create(ctx context.Context, inv domain.Invitation) (domain.Invitation, error) {
	if inv.RealmID() == "" || inv.Email() == "" {
		return nil, apierrors.InvalidArgument("realm_id and email are required")
	}
	if !inv.ExpiresAt().After(uc.now()) {
		return nil, apierrors.InvalidArgument("expires_at must be in the future")
	}
	token, err := randomToken()
	if err != nil {
		return nil, apierrors.InternalError("could not mint invitation token")
	}
	withToken := domain.NewInvitation(inv.RealmID(), inv.Email(), inv.ExpiresAt(),
		domain.WithInvitationInvitedBy(inv.InvitedBy()),
		domain.WithInvitationRoleID(inv.RoleID()),
		domain.WithInvitationResourceID(inv.ResourceID()),
		domain.WithInvitationTokenHash(hashCode(token)),
	)
	created, err := uc.invitations.Create(ctx, withToken)
	if err != nil {
		return nil, err
	}
	if err := uc.notifier.SendInvitation(ctx, created.Email(), token); err != nil {
		return nil, err
	}
	return created, nil
}

func (uc *invitationUsecase) Accept(ctx context.Context, token, accountID string) error {
	if token == "" || accountID == "" {
		return apierrors.InvalidArgument("token and account_id are required")
	}
	inv, err := uc.invitations.GetByTokenHash(ctx, hashCode(token))
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return apierrors.InvalidArgument("invalid invitation token")
		}
		return err
	}
	if inv.Status() != domain.InvitationStatusPending {
		return apierrors.New(apierrors.CodePreconditionFailed, apierrors.WithMessage("invitation is not pending"))
	}
	if !inv.ExpiresAt().After(uc.now()) {
		return apierrors.New(apierrors.CodePreconditionFailed, apierrors.WithMessage("invitation has expired"))
	}
	if inv.RoleID() != "" && inv.ResourceID() != "" {
		_, err := uc.bindings.Create(ctx, domain.NewBinding(
			inv.ResourceID(), inv.RoleID(), domain.SubjectTypeAccount, accountID))
		if err != nil {
			return err
		}
	}
	return uc.invitations.MarkAccepted(ctx, inv.ID(), uc.now())
}
