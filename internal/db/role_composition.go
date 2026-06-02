package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

var roleCompositionFieldMapping = map[string]string{
	fields.RoleID: "role_id",
}

// roleCompositionEntity omits the row id — the resolver and admin surface key
// composition by (role_id, component_role_id), never by the synthetic id.
type roleCompositionEntity struct {
	ERoleID          string `gorm:"column:role_id;type:uuid"`
	EComponentRoleID string `gorm:"column:component_role_id;type:uuid"`
	EOperator        string `gorm:"column:operator"`
	EOrdinal         int    `gorm:"column:ordinal"`
}

func (roleCompositionEntity) TableName() string { return "aegis.role_composition" }

type roleCompositionRepo struct {
	*postgres.Repo
}

func NewRoleCompositionRepository(db *gormdb.DBClient) (*roleCompositionRepo, error) {
	r, err := postgres.NewRepo(db, roleCompositionFieldMapping)
	if err != nil {
		return nil, err
	}
	return &roleCompositionRepo{Repo: r}, nil
}

// DeleteByRole clears roleID's composition. Pair with CreateMany inside a
// usecase transaction for an atomic overwrite.
func (r *roleCompositionRepo) DeleteByRole(ctx context.Context, roleID string) error {
	q := query.New(query.FilterBy(filter.OpEq, fields.RoleID, roleID))
	if err := r.QueryApply(ctx, q).Delete(&roleCompositionEntity{}).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

func (r *roleCompositionRepo) CreateMany(ctx context.Context, roleID string, components []domain.RoleComponent) error {
	if len(components) == 0 {
		return nil
	}
	rows := make([]roleCompositionEntity, 0, len(components))
	for _, c := range components {
		rows = append(rows, roleCompositionEntity{
			ERoleID:          roleID,
			EComponentRoleID: c.ComponentRoleID,
			EOperator:        string(c.Operator),
			EOrdinal:         c.Ordinal,
		})
	}
	if err := r.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

func (r *roleCompositionRepo) ListComponents(ctx context.Context, roleID string) ([]domain.RoleComponent, error) {
	q := query.New(query.FilterBy(filter.OpEq, fields.RoleID, roleID))
	var rows []roleCompositionEntity
	if err := r.QueryApply(ctx, q).Order("ordinal").Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	return toComponents(rows), nil
}

// ListAll returns every composition as role → ordered components, for the
// resolver.
func (r *roleCompositionRepo) ListAll(ctx context.Context) (map[string][]domain.RoleComponent, error) {
	var rows []roleCompositionEntity
	if err := r.DB.WithContext(ctx).Order("role_id, ordinal").Find(&rows).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := map[string][]domain.RoleComponent{}
	for _, row := range rows {
		out[row.ERoleID] = append(out[row.ERoleID], component(row))
	}
	return out, nil
}

func toComponents(rows []roleCompositionEntity) []domain.RoleComponent {
	out := make([]domain.RoleComponent, 0, len(rows))
	for _, row := range rows {
		out = append(out, component(row))
	}
	return out
}

func component(row roleCompositionEntity) domain.RoleComponent {
	return domain.RoleComponent{
		ComponentRoleID: row.EComponentRoleID,
		Operator:        domain.CompositionOperator(row.EOperator),
		Ordinal:         row.EOrdinal,
	}
}
