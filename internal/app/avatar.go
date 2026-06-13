package app

import (
	"context"
	"net/http"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

// maxAvatarBytes caps stored images so a BYTEA column can't be abused as bulk
// storage. 1 MiB comfortably fits a high-res avatar/logo.
const maxAvatarBytes = 1 << 20

// allowedImageTypes is the set of content types (as sniffed from the bytes, not
// trusted from the client) we accept for avatars/logos.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// AvatarStore persists avatar/logo blobs (implemented by the db package).
type AvatarStore interface {
	SetAccountAvatar(ctx context.Context, accountID string, image []byte, contentType string) error
	GetAccountAvatar(ctx context.Context, accountID string) (image []byte, contentType string, found bool, err error)
	DeleteAccountAvatar(ctx context.Context, accountID string) error
	SetOrgLogo(ctx context.Context, orgID string, image []byte, contentType string) error
	GetOrgLogo(ctx context.Context, orgID string) (image []byte, contentType string, found bool, err error)
	DeleteOrgLogo(ctx context.Context, orgID string) error
}

// AvatarUsecase validates and stores account avatars and organization logos.
// Authorization (self / org-owner) is enforced by the transport layer.
type AvatarUsecase interface {
	SetAccountAvatar(ctx context.Context, accountID string, image []byte) error
	GetAccountAvatar(ctx context.Context, accountID string) (image []byte, contentType string, found bool, err error)
	DeleteAccountAvatar(ctx context.Context, accountID string) error
	SetOrgLogo(ctx context.Context, orgID string, image []byte) error
	GetOrgLogo(ctx context.Context, orgID string) (image []byte, contentType string, found bool, err error)
	DeleteOrgLogo(ctx context.Context, orgID string) error
}

type avatarUsecase struct {
	store AvatarStore
}

func NewAvatarUsecase(store AvatarStore) AvatarUsecase {
	return &avatarUsecase{store: store}
}

// validateImage caps the size and derives the content type by sniffing the
// bytes (never trusting the client's header), rejecting anything that isn't a
// supported raster image.
func validateImage(image []byte) (string, error) {
	if len(image) == 0 {
		return "", apierrors.InvalidArgument("image is empty")
	}
	if len(image) > maxAvatarBytes {
		return "", apierrors.InvalidArgument("image exceeds the 1 MiB limit")
	}
	ct := http.DetectContentType(image)
	if !allowedImageTypes[ct] {
		return "", apierrors.InvalidArgument("unsupported image type; use JPEG, PNG, WebP, or GIF")
	}
	return ct, nil
}

func (u *avatarUsecase) SetAccountAvatar(ctx context.Context, accountID string, image []byte) error {
	ct, err := validateImage(image)
	if err != nil {
		return err
	}
	return u.store.SetAccountAvatar(ctx, accountID, image, ct)
}

func (u *avatarUsecase) GetAccountAvatar(ctx context.Context, accountID string) ([]byte, string, bool, error) {
	return u.store.GetAccountAvatar(ctx, accountID)
}

func (u *avatarUsecase) DeleteAccountAvatar(ctx context.Context, accountID string) error {
	return u.store.DeleteAccountAvatar(ctx, accountID)
}

func (u *avatarUsecase) SetOrgLogo(ctx context.Context, orgID string, image []byte) error {
	ct, err := validateImage(image)
	if err != nil {
		return err
	}
	return u.store.SetOrgLogo(ctx, orgID, image, ct)
}

func (u *avatarUsecase) GetOrgLogo(ctx context.Context, orgID string) ([]byte, string, bool, error) {
	return u.store.GetOrgLogo(ctx, orgID)
}

func (u *avatarUsecase) DeleteOrgLogo(ctx context.Context, orgID string) error {
	return u.store.DeleteOrgLogo(ctx, orgID)
}
