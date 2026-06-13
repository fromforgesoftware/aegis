package db

import (
	"context"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
)

// blobStore is a tiny key→image table accessor. The id column name differs per
// table (account_id / organization_id), so it's passed per call. Only raw SQL is
// used (the kit query DSL isn't needed for a 1:1 blob), so no field mapping.
type blobStore struct {
	*postgres.Repo
	table string
}

func newBlobStore(db *gormdb.DBClient, table string) (*blobStore, error) {
	r, err := postgres.NewRepo(db, map[string]string{})
	if err != nil {
		return nil, err
	}
	return &blobStore{Repo: r, table: table}, nil
}

func (s *blobStore) set(ctx context.Context, idCol, id string, image []byte, contentType string) error {
	sql := "INSERT INTO " + s.table + " (" + idCol + ", image, content_type, updated_at) " +
		"VALUES (?, ?, ?, NOW()) ON CONFLICT (" + idCol + ") DO UPDATE SET " +
		"image = EXCLUDED.image, content_type = EXCLUDED.content_type, updated_at = NOW()"
	if err := s.DB.WithContext(ctx).Exec(sql, id, image, contentType).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

type blobRow struct {
	Image       []byte
	ContentType string
}

func (s *blobStore) get(ctx context.Context, idCol, id string) ([]byte, string, bool, error) {
	var row blobRow
	// gorm Raw().Scan leaves the destination zero-valued (no error) when no row
	// matches, so an empty image means "not found".
	if err := s.DB.WithContext(ctx).
		Raw("SELECT image, content_type FROM "+s.table+" WHERE "+idCol+" = ?", id).
		Scan(&row).Error; err != nil {
		return nil, "", false, postgres.NewErrUnknown(err)
	}
	if len(row.Image) == 0 {
		return nil, "", false, nil
	}
	return row.Image, row.ContentType, true, nil
}

func (s *blobStore) del(ctx context.Context, idCol, id string) error {
	if err := s.DB.WithContext(ctx).Exec("DELETE FROM "+s.table+" WHERE "+idCol+" = ?", id).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// AvatarStore persists account avatars and organization logos in their dedicated
// blob tables. It implements app.AvatarStore.
type AvatarStore struct {
	accounts *blobStore
	orgs     *blobStore
}

func NewAvatarStore(db *gormdb.DBClient) (*AvatarStore, error) {
	accounts, err := newBlobStore(db, "aegis.account_avatar")
	if err != nil {
		return nil, err
	}
	orgs, err := newBlobStore(db, "aegis.organization_logo")
	if err != nil {
		return nil, err
	}
	return &AvatarStore{accounts: accounts, orgs: orgs}, nil
}

func (s *AvatarStore) SetAccountAvatar(ctx context.Context, accountID string, image []byte, contentType string) error {
	return s.accounts.set(ctx, "account_id", accountID, image, contentType)
}

func (s *AvatarStore) GetAccountAvatar(ctx context.Context, accountID string) ([]byte, string, bool, error) {
	return s.accounts.get(ctx, "account_id", accountID)
}

func (s *AvatarStore) DeleteAccountAvatar(ctx context.Context, accountID string) error {
	return s.accounts.del(ctx, "account_id", accountID)
}

func (s *AvatarStore) SetOrgLogo(ctx context.Context, orgID string, image []byte, contentType string) error {
	return s.orgs.set(ctx, "organization_id", orgID, image, contentType)
}

func (s *AvatarStore) GetOrgLogo(ctx context.Context, orgID string) ([]byte, string, bool, error) {
	return s.orgs.get(ctx, "organization_id", orgID)
}

func (s *AvatarStore) DeleteOrgLogo(ctx context.Context, orgID string) error {
	return s.orgs.del(ctx, "organization_id", orgID)
}
