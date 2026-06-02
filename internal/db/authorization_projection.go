package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
)

// authorizationProjectionRepo drives the effective_authorizations materialised
// view. It owns no entity of its own — it invokes the SECURITY DEFINER refresh
// function so the runtime role can rebuild the projection without owning it.
type authorizationProjectionRepo struct {
	db *gormdb.DBClient
}

func NewAuthorizationProjectionRepository(db *gormdb.DBClient) *authorizationProjectionRepo {
	return &authorizationProjectionRepo{db: db}
}

func (r *authorizationProjectionRepo) Refresh(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Exec("SELECT aegis.refresh_effective_authorizations()").Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
