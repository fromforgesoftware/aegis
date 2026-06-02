package app_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/persistence/persistencetest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestRoleResolver_WritesComposedSets(t *testing.T) {
	links := apptest.NewRolePermissionRepository(t)
	compositions := apptest.NewRoleCompositionRepository(t)
	inheritance := apptest.NewPermissionInheritanceRepository(t)
	effective := apptest.NewRoleEffectivePermissionRepository(t)
	uc := app.NewRoleResolver(links, compositions, inheritance, effective, persistencetest.NewTransactioner())

	links.EXPECT().ListAll(mock.Anything).Return(map[string][]string{
		"editor": {"doc.write"},
		"viewer": {"doc.read"},
	}, nil)
	compositions.EXPECT().ListAll(mock.Anything).Return(map[string][]domain.RoleComponent{
		"editor": {{ComponentRoleID: "viewer", Operator: domain.CompositionUnion, Ordinal: 0}},
	}, nil)
	inheritance.EXPECT().ListAllEdges(mock.Anything).Return(map[string][]string{}, nil)

	effective.EXPECT().DeleteAll(mock.Anything).Return(nil)
	// editor = its own doc.write ∪ viewer's doc.read.
	effective.EXPECT().CreateMany(mock.Anything, "editor", []string{"doc.read", "doc.write"}).Return(nil)
	effective.EXPECT().CreateMany(mock.Anything, "viewer", []string{"doc.read"}).Return(nil)

	require.NoError(t, uc.Resolve(context.Background()))
}
