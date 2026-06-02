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
)

func newInvitationUsecase(t *testing.T) (
	*apptest.InvitationRepository,
	*apptest.BindingUsecase,
	*apptest.NotificationSender,
	app.InvitationUsecase,
) {
	invites := apptest.NewInvitationRepository(t)
	bindings := apptest.NewBindingUsecase(t)
	notifier := apptest.NewNotificationSender(t)
	uc := app.NewInvitationUsecase(invites, bindings, notifier)
	return invites, bindings, notifier, uc
}

func TestInvitationCreate_MintsTokenAndSends(t *testing.T) {
	invites, _, notifier, uc := newInvitationUsecase(t)
	// The persisted invitation carries a token hash, never the raw token.
	invites.EXPECT().Create(mock.Anything, mock.MatchedBy(func(i domain.Invitation) bool {
		return i.Email() == "new@x.com" && i.TokenHash() != ""
	})).Return(domain.NewInvitation("r", "new@x.com", time.Now().Add(time.Hour),
		domain.WithInvitationID("inv-1")), nil)
	notifier.EXPECT().SendInvitation(mock.Anything, "new@x.com", mock.Anything).Return(nil)

	_, err := uc.Create(context.Background(), domain.NewInvitation("r", "new@x.com", time.Now().Add(time.Hour),
		domain.WithInvitationRoleID("role-1"), domain.WithInvitationResourceID("res-1")))
	require.NoError(t, err)
}

func TestInvitationCreate_RejectsPastExpiry(t *testing.T) {
	_, _, _, uc := newInvitationUsecase(t)
	_, err := uc.Create(context.Background(), domain.NewInvitation("r", "new@x.com", time.Now().Add(-time.Hour)))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestInvitationAccept_BindsAndMarksAccepted(t *testing.T) {
	invites, bindings, _, uc := newInvitationUsecase(t)
	invites.EXPECT().GetByTokenHash(mock.Anything, mock.Anything).
		Return(domain.NewInvitation("r", "new@x.com", time.Now().Add(time.Hour),
			domain.WithInvitationID("inv-1"),
			domain.WithInvitationRoleID("role-1"), domain.WithInvitationResourceID("res-1")), nil)
	bindings.EXPECT().Create(mock.Anything, mock.MatchedBy(func(b domain.Binding) bool {
		return b.ResourceID() == "res-1" && b.RoleID() == "role-1" &&
			b.SubjectType() == domain.SubjectTypeAccount && b.SubjectID() == "acc-1"
	})).Return(domain.NewBinding("res-1", "role-1", domain.SubjectTypeAccount, "acc-1"), nil)
	invites.EXPECT().MarkAccepted(mock.Anything, "inv-1", mock.Anything).Return(nil)

	require.NoError(t, uc.Accept(context.Background(), "raw-token", "acc-1"))
}

func TestInvitationAccept_RejectsExpired(t *testing.T) {
	invites, _, _, uc := newInvitationUsecase(t)
	invites.EXPECT().GetByTokenHash(mock.Anything, mock.Anything).
		Return(domain.NewInvitation("r", "new@x.com", time.Now().Add(-time.Hour),
			domain.WithInvitationID("inv-1")), nil)

	err := uc.Accept(context.Background(), "raw-token", "acc-1")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodePreconditionFailed))
}

func TestInvitationAccept_RejectsAlreadyAccepted(t *testing.T) {
	invites, _, _, uc := newInvitationUsecase(t)
	invites.EXPECT().GetByTokenHash(mock.Anything, mock.Anything).
		Return(domain.NewInvitation("r", "new@x.com", time.Now().Add(time.Hour),
			domain.WithInvitationID("inv-1"), domain.WithInvitationStatus(domain.InvitationStatusAccepted)), nil)

	err := uc.Accept(context.Background(), "raw-token", "acc-1")
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodePreconditionFailed))
}
