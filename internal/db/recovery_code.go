package db

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
)

type recoveryCodeEntity struct {
	EID        string     `gorm:"column:id;type:uuid;default:uuid_generate_v4()"`
	ECreatedAt time.Time  `gorm:"column:created_at;autoCreateTime:true"`
	EAccountID string     `gorm:"column:account_id;type:uuid"`
	ECodeHash  string     `gorm:"column:code_hash"`
	EUsedAt    *time.Time `gorm:"column:used_at"`
}

func (recoveryCodeEntity) TableName() string { return "aegis.recovery_code" }

type recoveryCodeRepo struct {
	*postgres.Repo
}

func NewRecoveryCodeRepository(db *gormdb.DBClient) (*recoveryCodeRepo, error) {
	r, err := postgres.NewRepo(db, map[string]string{})
	if err != nil {
		return nil, err
	}
	return &recoveryCodeRepo{Repo: r}, nil
}

// DeleteByAccount clears an account's codes; pair with CreateMany in a usecase
// transaction to regenerate the batch atomically.
func (r *recoveryCodeRepo) DeleteByAccount(ctx context.Context, accountID string) error {
	if err := r.DB.WithContext(ctx).
		Exec("DELETE FROM aegis.recovery_code WHERE account_id = ?", accountID).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

func (r *recoveryCodeRepo) CreateMany(ctx context.Context, accountID string, codeHashes []string) error {
	if len(codeHashes) == 0 {
		return nil
	}
	rows := make([]recoveryCodeEntity, 0, len(codeHashes))
	for _, h := range codeHashes {
		rows = append(rows, recoveryCodeEntity{EAccountID: accountID, ECodeHash: h})
	}
	if err := r.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// Consume marks a matching unused code used, returning true if one was spent.
// One-time semantics come from the used_at IS NULL guard in the UPDATE.
func (r *recoveryCodeRepo) Consume(ctx context.Context, accountID, codeHash string, at time.Time) (bool, error) {
	tx := r.DB.WithContext(ctx).Exec(
		"UPDATE aegis.recovery_code SET used_at = ? WHERE account_id = ? AND code_hash = ? AND used_at IS NULL",
		at, accountID, codeHash)
	if tx.Error != nil {
		return false, postgres.NewErrUnknown(tx.Error)
	}
	return tx.RowsAffected == 1, nil
}
