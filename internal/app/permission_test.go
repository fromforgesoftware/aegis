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
)

func newPermissionUsecase(t *testing.T) (
	*apptest.PermissionRepository,
	*apptest.PermissionInheritanceRepository,
	app.PermissionUsecase,
) {
	repo := apptest.NewPermissionRepository(t)
	inheritance := apptest.NewPermissionInheritanceRepository(t)
	uc := app.NewPermissionUsecase(repo, inheritance, persistencetest.NewTransactioner())
	return repo, inheritance, uc
}

func TestPermissionCreate_RoundsTripsTheSlug(t *testing.T) {
	repo, _, uc := newPermissionUsecase(t)
	want := domain.NewPermission("doc.read", "doc", "read")
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p domain.Permission) bool {
		return p.ID() == "doc.read" && p.ResourceType() == "doc" && p.Verb() == "read"
	})).Return(want, nil)

	got, err := uc.Create(context.Background(), domain.NewPermission("doc.read", "doc", "read"))
	require.NoError(t, err)
	assert.Equal(t, "doc.read", got.ID())
}

func TestPermissionCreate_RejectsMismatchedSlug(t *testing.T) {
	// "doc.read" must decompose into resource_type=doc, verb=read; mismatched
	// catalog entries would corrupt later lookups by (resource_type, verb).
	_, _, uc := newPermissionUsecase(t)
	_, err := uc.Create(context.Background(), domain.NewPermission("doc.read", "trade", "place"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestPermissionCreate_RejectsEmptyFields(t *testing.T) {
	_, _, uc := newPermissionUsecase(t)
	_, err := uc.Create(context.Background(), domain.NewPermission("", "doc", "read"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestPermissionCreate_RejectsDotInResourceTypeOrVerb(t *testing.T) {
	_, _, uc := newPermissionUsecase(t)
	// A resource_type with a dot in it would collide with the slug parser.
	_, err := uc.Create(context.Background(), domain.NewPermission("a.b.c", "a.b", "c"))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestPermissionSetImplications_OverwritesAtomically(t *testing.T) {
	repo, inheritance, uc := newPermissionUsecase(t)
	repo.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.write", "doc", "write"), nil).Once()
	repo.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.read", "doc", "read"), nil).Once()
	inheritance.EXPECT().DeleteByPermission(mock.Anything, "doc.write").Return(nil)
	inheritance.EXPECT().CreateMany(mock.Anything, "doc.write", []string{"doc.read"}).Return(nil)

	require.NoError(t, uc.SetImplications(context.Background(), "doc.write", []string{"doc.read"}))
}

func TestPermissionSetImplications_RejectsSelfImplication(t *testing.T) {
	repo, _, uc := newPermissionUsecase(t)
	repo.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.write", "doc", "write"), nil)

	err := uc.SetImplications(context.Background(), "doc.write", []string{"doc.write"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestPermissionSetImplications_RejectsUnknownImplied(t *testing.T) {
	repo, _, uc := newPermissionUsecase(t)
	repo.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewPermission("doc.write", "doc", "write"), nil).Once()
	repo.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, apierrors.NotFound("permission", "ghost")).Once()

	err := uc.SetImplications(context.Background(), "doc.write", []string{"doc.ghost"})
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}
