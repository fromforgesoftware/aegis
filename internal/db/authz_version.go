package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
)

// authzVersionRepo reads and publishes the read-after-write counters. The
// write_version is advanced by DB triggers on the source tables; this repo
// only reads it and records the projection_version at refresh time.
type authzVersionRepo struct {
	db *gormdb.DBClient
}

func NewAuthzVersionRepository(db *gormdb.DBClient) *authzVersionRepo {
	return &authzVersionRepo{db: db}
}

func (r *authzVersionRepo) Versions(ctx context.Context) (write, projection int64, err error) {
	var row struct {
		WriteVersion      int64 `gorm:"column:write_version"`
		ProjectionVersion int64 `gorm:"column:projection_version"`
	}
	if err := r.db.WithContext(ctx).
		Raw("SELECT write_version, projection_version FROM aegis.authz_version").
		Scan(&row).Error; err != nil {
		return 0, 0, postgres.NewErrUnknown(err)
	}
	return row.WriteVersion, row.ProjectionVersion, nil
}

// PublishProjection records writeVersion as the version the projection now
// reflects. Called after a resolve + refresh with the write_version captured
// before that cycle began.
func (r *authzVersionRepo) PublishProjection(ctx context.Context, writeVersion int64) error {
	if err := r.db.WithContext(ctx).
		Exec("UPDATE aegis.authz_version SET projection_version = ?", writeVersion).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
