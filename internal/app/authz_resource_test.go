package app_test

import (
	"context"
	"testing"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestAuthzResourceCreate_TopLevel(t *testing.T) {
	repo := apptest.NewAuthzResourceRepository(t)
	want := internaltest.NewAuthzResource(
		internaltest.WithAuthzResourceRealmID("r"),
		internaltest.WithAuthzResourceResourceType("workspace"),
	)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(internaltest.MatchAuthzResource(want))).
		Return(internaltest.NewAuthzResource(internaltest.WithAuthzResourceID("ws-1"),
			internaltest.WithAuthzResourceRealmID("r"),
			internaltest.WithAuthzResourceResourceType("workspace")), nil)

	uc := app.NewAuthzResourceUsecase(repo)
	got, err := uc.Create(context.Background(),
		internaltest.NewAuthzResource(internaltest.WithAuthzResourceRealmID("r"),
			internaltest.WithAuthzResourceResourceType("workspace")))
	require.NoError(t, err)
	assert.Equal(t, "ws-1", got.ID())
}

func TestAuthzResourceCreate_WithParent(t *testing.T) {
	repo := apptest.NewAuthzResourceRepository(t)
	// Parent lookup happens before Create so the parent-realm invariant can be
	// enforced.
	repo.EXPECT().Get(mock.Anything, mock.Anything).Return(internaltest.NewAuthzResource(
		internaltest.WithAuthzResourceID("ws-1"),
		internaltest.WithAuthzResourceRealmID("r"),
		internaltest.WithAuthzResourceResourceType("workspace"),
	), nil)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.AuthzResource) bool {
		return r.ParentID() == "ws-1" && r.ResourceType() == "doc"
	})).Return(internaltest.NewAuthzResource(
		internaltest.WithAuthzResourceID("doc-1"),
		internaltest.WithAuthzResourceRealmID("r"),
		internaltest.WithAuthzResourceResourceType("doc"),
		internaltest.WithAuthzResourceParentID("ws-1"),
	), nil)

	uc := app.NewAuthzResourceUsecase(repo)
	_, err := uc.Create(context.Background(),
		internaltest.NewAuthzResource(
			internaltest.WithAuthzResourceRealmID("r"),
			internaltest.WithAuthzResourceResourceType("doc"),
			internaltest.WithAuthzResourceParentID("ws-1"),
		))
	require.NoError(t, err)
}

func TestAuthzResourceCreate_RejectsCrossRealmParent(t *testing.T) {
	// A doc in realm "r" can't claim a workspace in realm "other-r" as parent
	// — the closure walk would leak grants across the realm boundary.
	repo := apptest.NewAuthzResourceRepository(t)
	repo.EXPECT().Get(mock.Anything, mock.Anything).Return(internaltest.NewAuthzResource(
		internaltest.WithAuthzResourceID("ws-1"),
		internaltest.WithAuthzResourceRealmID("other-r"),
	), nil)

	uc := app.NewAuthzResourceUsecase(repo)
	_, err := uc.Create(context.Background(),
		internaltest.NewAuthzResource(
			internaltest.WithAuthzResourceRealmID("r"),
			internaltest.WithAuthzResourceResourceType("doc"),
			internaltest.WithAuthzResourceParentID("ws-1"),
		))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestAuthzResourceCreate_RejectsUnknownParent(t *testing.T) {
	repo := apptest.NewAuthzResourceRepository(t)
	repo.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, apierrors.NotFound("resource", "ghost"))

	uc := app.NewAuthzResourceUsecase(repo)
	_, err := uc.Create(context.Background(),
		internaltest.NewAuthzResource(
			internaltest.WithAuthzResourceRealmID("r"),
			internaltest.WithAuthzResourceResourceType("doc"),
			internaltest.WithAuthzResourceParentID("ghost"),
		))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
}

func TestAuthzResourceCreate_Validates(t *testing.T) {
	cases := map[string]domain.AuthzResource{
		"empty realm":    domain.NewAuthzResource("", "doc"),
		"empty type":     domain.NewAuthzResource("r", ""),
		"bad visibility": domain.NewAuthzResource("r", "doc", domain.WithAuthzResourceVisibility(domain.Visibility("BOGUS"))),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			uc := app.NewAuthzResourceUsecase(apptest.NewAuthzResourceRepository(t))
			_, err := uc.Create(context.Background(), in)
			require.Error(t, err)
			assert.True(t, apierrors.Is(err, apierrors.CodeInvalidArgument))
		})
	}
}
