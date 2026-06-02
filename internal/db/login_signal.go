package db

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"

	"github.com/fromforgesoftware/aegis/internal/app"
)

type loginSignalEntity struct {
	EID        string    `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	ECreatedAt time.Time `gorm:"column:created_at;autoCreateTime:true"`
	EAccountID string    `gorm:"column:account_id;type:uuid"`
	ERealmID   string    `gorm:"column:realm_id;type:uuid"`
	EIP        string    `gorm:"column:ip"`
	EDeviceID  string    `gorm:"column:device_id"`
	ESucceeded bool      `gorm:"column:succeeded"`
}

func (loginSignalEntity) TableName() string { return "aegis.login_signal" }

type loginSignalRepo struct {
	db *gormdb.DBClient
}

func NewLoginSignalRepository(db *gormdb.DBClient) *loginSignalRepo {
	return &loginSignalRepo{db: db}
}

func (r *loginSignalRepo) Record(ctx context.Context, s app.LoginSignal) error {
	e := &loginSignalEntity{
		EAccountID: s.AccountID, ERealmID: s.RealmID, EIP: s.IP, EDeviceID: s.DeviceID, ESucceeded: s.Succeeded,
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// SeenIP reports whether the account has a prior signal from this IP.
func (r *loginSignalRepo) SeenIP(ctx context.Context, accountID, ip string) (bool, error) {
	return r.exists(ctx, "account_id = ? AND ip = ?", accountID, ip)
}

// SeenDevice reports whether the account has a prior signal from this device.
// An empty device id is treated as never-seen (so it counts toward risk).
func (r *loginSignalRepo) SeenDevice(ctx context.Context, accountID, deviceID string) (bool, error) {
	if deviceID == "" {
		return false, nil
	}
	return r.exists(ctx, "account_id = ? AND device_id = ?", accountID, deviceID)
}

// RecentFailures counts failed attempts for the account at or after since.
func (r *loginSignalRepo) RecentFailures(ctx context.Context, accountID string, since time.Time) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Raw("SELECT COUNT(*) FROM aegis.login_signal WHERE account_id = ? AND succeeded = FALSE AND created_at >= ?",
			accountID, since).
		Scan(&count).Error
	if err != nil {
		return 0, postgres.NewErrUnknown(err)
	}
	return int(count), nil
}

func (r *loginSignalRepo) exists(ctx context.Context, where string, args ...any) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).
		Raw("SELECT EXISTS(SELECT 1 FROM aegis.login_signal WHERE "+where+")", args...).
		Scan(&found).Error
	if err != nil {
		return false, postgres.NewErrUnknown(err)
	}
	return found, nil
}
